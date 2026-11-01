// SPDX-License-Identifier: MPL-2.0

package conformance

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/oioio-space/inkwell/protocol/blob"
	"github.com/oioio-space/inkwell/protocol/bridge"
	"github.com/oioio-space/inkwell/protocol/invite"
	"github.com/oioio-space/inkwell/protocol/sig"
	"github.com/oioio-space/inkwell/protocol/trust"
)

// ErrUnsupported identifies a future case without an implementation in this release.
var ErrUnsupported = errors.New("conformance: decoder not implemented")

func fixture() (*trust.List, trust.ProjectID, error) {
	data, err := vectorsData("trust.json")
	if err != nil {
		return nil, trust.ProjectID{}, err
	}
	var f struct {
		Vectors []struct {
			ID    string `json:"id"`
			Input string `json:"input_hex"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, trust.ProjectID{}, err
	}
	var list *trust.List
	var id trust.ProjectID
	for _, v := range f.Vectors {
		if v.ID != "op-genesis" && v.ID != "op-add-writer" {
			continue
		}
		b, err := hex.DecodeString(v.Input)
		if err != nil {
			return nil, id, err
		}
		o, err := trust.Decode(b)
		if err != nil {
			return nil, id, err
		}
		if v.ID == "op-genesis" {
			id = trust.ProjectID(o.Hash())
			list = trust.New(id)
		}
		if list == nil {
			return nil, id, errors.New("missing genesis fixture")
		}
		if err := list.Apply(o); err != nil {
			return nil, id, err
		}
	}
	if list == nil || len(list.Members()) < 2 {
		return nil, id, errors.New("missing trust fixture members")
	}
	return list, id, nil
}

// Reference checks a vector against this release's codecs, signatures and fold.
func Reference(v Vector) error {
	list, id, err := fixture()
	if err != nil {
		return err
	}
	c := &referenceCase{vector: v, list: list, project: id}
	if _, ok := v.Fields["input_hex"]; ok {
		c.input = c.unhex(c.inputHex())
	}
	if _, ok := v.Fields["expect"]; ok {
		if err := v.Decode("expect", &c.expect); err != nil {
			return err
		}
		if c.expect == nil {
			return errors.New("null expectations")
		}
	}
	checks := map[string]func() error{
		"bulk":               c.bulk,
		"bridge_envelope":    c.bridge,
		"trust_op":           c.op,
		"trust_op+apply":     c.op,
		"trust_sync":         c.sync,
		"attestation":        c.attestation,
		"attestation+verify": c.attestation,
		"lobby_request":      c.request,
		"lobby_reply":        c.reply,
		"ticket_text":        c.ticket,
		"public_key":         c.key,
		"signature":          c.signature,
		"signature+key":      c.signature,
		"":                   c.signature,
		"peer_id":            c.peer,
		"trust_fold":         c.fold,
	}
	required := map[string][]string{
		"ticket_text": {"text"}, "peer_id": {"peer_id"},
		"signature":     {"public_hex", "signed_bytes_hex"},
		"signature+key": {"public_hex", "signed_bytes_hex", "signature_hex"},
		"":              {"public_hex", "signature_hex", "domain", "message_hex"},
		"trust_fold":    {"members"},
	}
	for _, k := range required[v.Decoder] {
		if raw, ok := c.expect[k]; !ok || bytes.Equal(raw, []byte("null")) {
			return fmt.Errorf("missing expectation %s", k)
		}
	}
	switch v.Decoder {
	case "ticket_text", "peer_id", "signature", "signature+key", "", "trust_fold":
	case "public_key":
		if _, ok := c.expect["keys_hex"]; !ok {
			c.input = c.unhex(c.inputHex())
		}
	default:
		c.input = c.unhex(c.inputHex())
	}
	check, ok := checks[v.Decoder]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnsupported, v.Decoder)
	}
	if err := check(); err != nil {
		return err
	}
	if c.fieldErr != nil {
		return c.fieldErr
	}
	if c.verdictChecked {
		return nil
	}
	if (c.err == nil) != (v.Verdict == "accept") {
		if c.err != nil {
			return fmt.Errorf("verdict %s: %w", v.Verdict, c.err)
		}
		return fmt.Errorf("expected %s, decoder accepted input", v.Verdict)
	}
	if c.err == nil && c.encoded != nil && !bytes.Equal(c.encoded, c.input) {
		return errors.New("binary round trip mismatch")
	}
	if c.err == nil && c.exp("blake3_hex") != "" && (len(c.input) < 33 || hex.EncodeToString(c.input[1:33]) != c.exp("blake3_hex")) {
		return errors.New("blob hash mismatch")
	}
	return nil
}

type referenceCase struct {
	vector         Vector
	input, encoded []byte
	list           *trust.List
	project        trust.ProjectID
	err            error
	fieldErr       error
	expect         map[string]json.RawMessage
	verdictChecked bool
}

func (c *referenceCase) unhex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		c.fieldErr = err
	}
	return b
}

func (c *referenceCase) inputHex() string {
	if bytes.Equal(c.vector.Fields["input_hex"], []byte("null")) {
		c.fieldErr = errors.New("null field input_hex")
	}
	var s string
	if err := c.vector.Decode("input_hex", &s); err != nil {
		c.fieldErr = err
	}
	return s
}

func (c *referenceCase) exp(k string) string {
	var s string
	if raw, ok := c.expect[k]; ok {
		if bytes.Equal(raw, []byte("null")) {
			c.fieldErr = fmt.Errorf("null expectation %s", k)
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			c.fieldErr = err
		}
	}
	return s
}

func (c *referenceCase) bulk() error {
	var m blob.Message
	m, c.err = blob.Decode(c.input)
	if c.err == nil {
		c.encoded, c.err = blob.Encode(m)
	}
	return nil
}

func (c *referenceCase) bridge() error {
	var e bridge.Envelope
	e, c.err = bridge.Decode(c.input)
	if c.err == nil {
		c.encoded, c.err = bridge.Encode(e)
	}
	return nil
}

func (c *referenceCase) op() error {
	var o trust.Op
	o, c.err = trust.Decode(c.input)
	if c.err == nil {
		c.encoded = trust.Encode(o)
		if h := c.exp("hash_hex"); h != "" && o.Hash().String() != h {
			return fmt.Errorf("hash mismatch")
		}
		if c.vector.Decoder == "trust_op+apply" {
			c.err = c.list.Apply(o)
		}
	}
	return nil
}

func (c *referenceCase) sync() error {
	_, c.err = trust.NewSync[string](c.list).OnMessage("conformance", c.input)
	return nil
}

func (c *referenceCase) attestation() error {
	var a trust.Attestation
	a, c.err = trust.DecodeAttestation(c.input)
	if c.err == nil {
		c.encoded = trust.EncodeAttestation(a)
		if c.vector.Decoder == "attestation+verify" {
			c.err = c.list.Verify(a)
		}
	}
	return nil
}

func (c *referenceCase) request() error {
	var r invite.Request
	r, c.err = invite.DecodeRequest(c.input)
	if c.err == nil {
		c.encoded = r.Encode()
	}
	return nil
}

func (c *referenceCase) reply() error {
	var r invite.Reply
	r, c.err = invite.DecodeReply(c.input)
	if c.err == nil {
		c.encoded = r.Encode()
	}
	return nil
}

func (c *referenceCase) ticket() error {
	var t invite.Ticket
	t, c.err = invite.ParseText(c.exp("text"))
	if c.err == nil && t.Text() != c.exp("text") {
		return errors.New("ticket round trip")
	}
	return nil
}

func (c *referenceCase) key() error {
	var keys []string
	if raw, ok := c.expect["keys_hex"]; ok {
		if err := json.Unmarshal(raw, &keys); err != nil {
			return err
		}
		if len(keys) == 0 {
			return errors.New("empty keys")
		}
	}
	if len(keys) == 0 {
		keys = []string{c.inputHex()}
	}
	for _, k := range keys {
		e := sig.ValidatePublicKey(c.unhex(k))
		if (e == nil) != (c.vector.Verdict == "accept") {
			return fmt.Errorf("public key verdict: %w", e)
		}
	}
	c.verdictChecked = true
	return nil
}

func (c *referenceCase) signature() error {
	key := c.unhex(c.exp("public_hex"))
	msg := c.unhex(c.exp("signed_bytes_hex"))
	signature := c.unhex(c.exp("signature_hex"))
	if _, ok := c.expect["signature_hex"]; !ok {
		signature = c.unhex(c.inputHex())
	}
	if c.vector.Decoder == "" {
		msg = append([]byte("inkwell/"+c.exp("domain")+"/v1\x00"), c.unhex(c.exp("message_hex"))...)
	}
	if len(key) != 32 || (c.vector.Decoder == "signature+key" && sig.ValidatePublicKey(key) != nil) || !ed25519.Verify(key, msg, signature) {
		c.err = errors.New("signature rejected")
	}
	return nil
}

func (c *referenceCase) peer() error {
	// Ed25519 identity multihash: 00 24 08 01 12 20 || key (base58btc).
	raw := base58(c.exp("peer_id"))
	if len(raw) != 38 || !bytes.Equal(raw[:6], []byte{0, 36, 8, 1, 18, 32}) {
		c.err = errors.New("peer ID envelope")
	} else {
		c.err = sig.ValidatePublicKey(raw[6:])
	}
	return nil
}

func (c *referenceCase) fold() error {
	var inputs []string
	if err := c.vector.Decode("inputs_hex", &inputs); err != nil {
		return err
	}
	if len(inputs) == 0 {
		return errors.New("empty fold inputs")
	}
	var wanted []struct {
		Public    string     `json:"public_hex"`
		Role      trust.Role `json:"role"`
		Suspended bool       `json:"suspended"`
	}
	if err := json.Unmarshal(c.expect["members"], &wanted); err != nil {
		return err
	}
	if len(wanted) == 0 {
		return errors.New("empty expected members")
	}
	for pass := 0; pass < 2; pass++ {
		fold := trust.New(c.project)
		ops := make([]trust.Op, 0, len(inputs))
		for _, s := range inputs {
			o, e := trust.Decode(c.unhex(s))
			if e != nil {
				return e
			}
			ops = append(ops, o)
		}
		if pass == 1 {
			slices.Reverse(ops)
		}
		for range ops {
			_, _ = fold.ApplyAll(ops)
		}
		got := fold.Members()
		if len(got) == 0 || len(got) != len(wanted) {
			return errors.New("fold member count")
		}
		for i, m := range got {
			if hex.EncodeToString(m.Pub) != wanted[i].Public || m.Role != wanted[i].Role || m.Suspended != wanted[i].Suspended {
				return errors.New("fold member mismatch")
			}
		}
	}
	return nil
}
