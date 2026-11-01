# Inkwell protocol

This MPL-2.0 Go module contains the existing binary formats used by Inkwell:
membership operations and attestations (`trust`), invitation tickets and lobby
messages (`invite`), blob pull frames and BLAKE3 identifiers (`blob`), and the
internal webview envelope (`bridge`). `sig` contains the existing Ed25519 key
validator. The root package exports the specification §16 bounds and opaque peer
identifiers. It has no dependency on the application, libp2p, Wails or Automerge.

Version: **v0.1.0**, tagged as `protocol/v0.1.0`. The protocol specification is
still a draft. Extraction preserves the existing bytes; the constants for future
sync2 formats do not activate them. Membership folding and the existing gossip
state machine moved with the codecs because they share signed types and helpers.

Use `go get github.com/oioio-space/inkwell/protocol@v0.1.0`, then import a
subpackage. For example:

```go
frame, err := blob.Encode(blob.Want{Hash: blob.HashOf(data), Length: blob.GroupSize})
message, err := blob.Decode(frame)
```

Decoders enforce structure and bounds. Signed-object decoding alone does not
prove authority: apply operations to `trust.List` and verify attestations against
that list. A blob Slice's Bao proof must be verified against the requested root
and expected length by the consumer; decoding a frame alone does not do that.
See the runnable `Example*` functions with `go test ./...`.

## Conformance

`vectors.FS` embeds the canonical JSON corpus. Documentation points to this copy;
there are no copied JSON files or symlinks under `docs/protocol/vectors`.
The original JSON generator metadata is retained to preserve every corpus byte.
`conformance.Run(1, nil)` runs the Go reference. Pass your own `conformance.Check`
to test another implementation: return nil only when both the specified verdict
and expectations match. Every case is reported, and `Result.Required` distinguishes
this migration step from future requirements. Unimplemented future cases report
`ErrUnsupported`; they are never presented as passes. Check all required results.
The TypeScript runner in `@inkwell/protocol` reads the same files directly.

Run tests independently with `cd protocol && go test -race ./...`. Regenerate into
a temporary directory with `go run ./cmd/vectors OUT CASES_JSON` (the Chalkias
cases file is an explicit input). This branch has no policy implementation:
`policy.json` remains a preserved future fixture, not a generated codec claim.
The CBOR generator runs from the frontend using `pnpm run vectors:repo` and writes
JSON to stdout. Future proposed formats in these generators remain reference
fixtures until their migration steps; they are not enabled in the application.

## Go API (non-wire helpers)

These Go conveniences are not part of the wire format. In particular,
`blob.Hash.Hex` returns the lowercase filename used by `internal/blob`,
`blob.Hash.Short` abbreviates hashes for application logs, and `blob.HashPrefix`
is used by `internal/blob` to parse filenames as canonical hash text.

The export inventory below covers every public declaration and explicit public
struct field added by this extraction (including the codec API for the formats
in `docs/protocol/spec.md`). Names are relative to each package. Go names, error
values, constructors, accessors and runner metadata do not add wire fields.

- `protocol`: Shared §16 bounds, version and opaque peer validation let consumers enforce the specification without application dependencies.
  `AnnouncementQuotaSlack`, `AttestWantSentPerPeriod`, `AttestWantServedPerPeriod`, `AttestWantTimeout`, `AttestationBytes`, `AttestationDistinctHeads`, `AttestationKeys`, `AuthorPendingMessages`, `AuthorPendingTimeout`, `BridgeInlineBytes`, `BridgeLargeObjects`, `BulkBanDuration`, `BulkBusyPerMinute`, `BulkConsecutiveBusy`, `BulkFaultsBeforeBan`, `BulkGlobalServesPerPeer`, `BulkIdleTimeout`, `BulkMinGlobalServes`, `BulkRangeFailures`, `BulkRequestsPerPeer`, `BulkRequestsPerPeerHash`, `BulkServesPerPeer`, `BulkStreams`, `CBORMaxArray`, `CBORMaxDepth`, `CBORMaxItems`, `CBORMaxMap`, `ClosingRepoStreams`, `ConcurrentBlobTransfers`, `ControlInputBytes`, `ControlOutputBytes`, `ControlStreams`, `CoverageDocuments`, `CoverageDocumentsPerSigner`, `CoverageHeads`, `CoveragePendingBytesPerPeer`, `CoveragePendingMessages`, `CoveragePendingTimeout`, `DocumentPendingBytesPerPeer`, `DocumentPendingMessages`, `EphemeralBurst`, `EphemeralPerSecond`, `EvidencePerKey`, `EvidencePerKeyEpoch`, `GenerationBackoffMemory`, `GenerationInitialBackoff`, `GenerationMaxBackoff`, `GuardWorkerBytes`, `HandshakeBurst`, `HandshakeInterval`, `HelloTimeout`, `HistoryRequestInterval`, `IgnorableBytesPerMinute`, `IgnorableFramesPerMinute`, `IncomingRepoStreamsPerPeriod`, `IndivisibleDiskBytes`, `IndivisibleIndexHistoryBytes`, `IndivisibleRanksPerPeer`, `IndivisibleSlotBytes`, `InitialSyncDocumentsPerPeer`, `LargeAnnouncementTimeout`, `LargeAnnouncementsPerPeer`, `LargeIdleTimeout`, `LargeReservationsPerPeer`, `LobbyProcessingTimeout`, `LobbyRequestTimeout`, `LobbyStreams`, `MaxAcknowledgmentBytes`, `MaxAnnouncementBytes`, `MaxAttestationBytes`, `MaxBulkFrame`, `MaxChangeOperations`, `MaxConnectedMembers`, `MaxConnectionsPerPeer`, `MaxDecisionBytes`, `MaxDecisions`, `MaxDecisionsPerAuthor`, `MaxDecompressedChangeBytes`, `MaxDocumentIDBytes`, `MaxEntries`, `MaxEphemeralBytes`, `MaxExactInteger`, `MaxFrame`, `MaxGeneration`, `MaxHandshakeBytes`, `MaxHaveHeads`, `MaxHeads`, `MaxHello`, `MaxInboundConnections`, `MaxIndivisibleBytes`, `MaxLargeBytes`, `MaxLobbyReply`, `MaxLobbyRequest`, `MaxMemberOps`, `MaxNetEnvelopeBytes`, `MaxOpBytes`, `MaxOutboundConnections`, `MaxParentPendingObjects`, `MaxParents`, `MaxPeerIDBytes`, `MaxPendingResets`, `MaxPresenceEntries`, `MaxPresentFrameBytes`, `MaxPresentWindows`, `MaxPreviewFrameBytes`, `MaxPreviewPages`, `MaxPreviewRequestBytes`, `MaxProjectURLBytes`, `MaxSessionsPerPeer`, `MaxStoreKeyComponentBytes`, `MaxStoreKeyComponents`, `MaxStorePageBytes`, `MaxStoreValueBytes`, `MaxSyncFrame`, `MaxTicketCharacters`, `ParentPendingPeriods`, `PeerID`, `PendingAnnouncementsPerAuthor`, `PendingCreditMessagesPerDocument`, `PresenceStreams`, `PresentBytesPerWindow`, `PresentFramesPerWindow`, `QUICConnectionHeadroom`, `QUICInitialConnectionWindow`, `QUICInitialStreamWindow`, `QUICMaxConnectionWindow`, `QUICMaxStreamWindow`, `ReconcilePeriod`, `ReconnectBackoffMemory`, `ReconnectInitialBackoff`, `ReconnectMaxBackoff`, `RepoInputBytes`, `RepoInputFrames`, `RepoOutputBytesPerPeer`, `RepoStreams`, `ResetTimeout`, `SetCursorsPerPeerSet`, `SetCursorsPerPeriod`, `StoreLanes`, `TicketLifetime`, `UnknownSignerPendingPeriods`, `UnreadLargeStreams`, `ValidPeerID`, `Version`.

- `protocol/blob`: Codec types and bounds expose the blob pull format; hash text helpers support storage and diagnostics, and sentinel errors support rejection checks.
  `Busy`, `Busy.Hash`, `Busy.Retry`, `Decode`, `Encode`, `ErrBadHash`, `ErrMalformed`, `GroupSize`, `Hash`, `Hash.Hex`, `Hash.Short`, `Hash.String`, `HashOf`, `HashPrefix`, `HashSize`, `MaxSlice`, `Message`, `Missing`, `Missing.Hash`, `ParseHash`, `Slice`, `Slice.Bao`, `Slice.Hash`, `Slice.Length`, `Slice.Offset`, `Want`, `Want.Hash`, `Want.Length`, `Want.Offset`.

- `protocol/bridge`: Envelope and payload codecs expose the internal bridge format; kind names and sentinel errors support dispatch and diagnostics.
  `Attest`, `Attested`, `Decode`, `DecodeHeads`, `DecodeRoles`, `Encode`, `EncodeHeads`, `EncodeRoles`, `Envelope`, `Envelope.Kind`, `Envelope.Payload`, `Envelope.Peer`, `ErrMalformed`, `HashSize`, `Heads`, `Heads.Doc`, `Heads.Heads`, `Kind`, `Kind.String`, `MaxDoc`, `MaxHeads`, `MaxMembers`, `MemberRole`, `MemberRole.Peer`, `MemberRole.Role`, `MemberRole.Suspended`, `Message`, `PeerDown`, `PeerUp`, `Revoked`, `Roles`, `Sealed`, `StreamName`, `Suspended`.

- `protocol/conformance`: Runner callbacks, corpus metadata, results and field access let third-party implementations run the shared corpus and report unsupported cases.
  `Check`, `ErrUnsupported`, `Reference`, `Result`, `Result.Err`, `Result.Passed`, `Result.Required`, `Result.Vector`, `Run`, `Vector`, `Vector.Decode`, `Vector.Decoder`, `Vector.Enforce`, `Vector.Fields`, `Vector.File`, `Vector.ID`, `Vector.Required`, `Vector.Status`, `Vector.Verdict`.

- `protocol/invite`: Ticket and lobby codecs expose the invitation format; text, expiry and error helpers support user entry and validation.
  `DecodeReply`, `DecodeRequest`, `ErrMalformed`, `KindAsk`, `KindHello`, `KindRedeem`, `MaxReply`, `MaxRequest`, `MaxTextLen`, `ParseText`, `Reason`, `ReasonBusy`, `ReasonDeclined`, `ReasonFailed`, `ReasonInvalid`, `ReasonTooLarge`, `Reply`, `Reply.Encode`, `Reply.Kind`, `Reply.Machine`, `Reply.Ops`, `Reply.Reason`, `Reply.RootURL`, `Reply.Title`, `ReplyHello`, `ReplyKind`, `ReplyRefused`, `ReplyWelcome`, `Request`, `Request.Encode`, `Request.Kind`, `Request.Name`, `Request.Project`, `Request.Secret`, `RequestKind`, `Ticket`, `Ticket.Addrs`, `Ticket.Expired`, `Ticket.Expires`, `Ticket.Host`, `Ticket.Person`, `Ticket.Project`, `Ticket.Role`, `Ticket.Secret`, `Ticket.Text`, `TicketTTL`.

- `protocol/sig`: Public-key validation and its sentinel error let consumers reject invalid signing keys consistently.
  `ErrInvalidPublicKey`, `ValidatePublicKey`.

- `protocol/trust`: Signed-object codecs expose membership and attestations; list, folding, signing and sync helpers preserve the existing reference state machine and support authority checks.
  `Attestation`, `Attestation.Doc`, `Attestation.Heads`, `Attestation.Project`, `Attestation.Seq`, `Attestation.Sig`, `Attestation.Signer`, `Broadcast`, `Decode`, `DecodeAttestation`, `Encode`, `EncodeAttestation`, `ErrBadLamport`, `ErrBadSignature`, `ErrLastAdmin`, `ErrMalformed`, `ErrNoGenesis`, `ErrNotAdmin`, `ErrNotStanding`, `ErrNotWriter`, `ErrOutranked`, `ErrSuspended`, `ErrTooManyOps`, `ErrUnknownAuthor`, `ErrWrongProject`, `Genesis`, `Hash`, `Hash.String`, `List`, `List.Apply`, `List.ApplyAll`, `List.Frontier`, `List.Has`, `List.Issue`, `List.IssueBatch`, `List.IssueWith`, `List.Len`, `List.Members`, `List.Pending`, `List.Project`, `List.Role`, `List.Sealed`, `List.Since`, `List.Suspended`, `List.Verify`, `MaxAttestationSize`, `MaxDoc`, `MaxHeads`, `MaxMessage`, `MaxName`, `MaxOpSize`, `MaxPrev`, `MaxRootURL`, `Member`, `Member.Name`, `Member.Person`, `Member.Pub`, `Member.ResumedBy`, `Member.Role`, `Member.Suspended`, `Member.SuspendedBy`, `Member.SuspendedOp`, `New`, `NewAttestation`, `NewSync`, `OnMessage`, `OnPeerDown`, `OnPeerUp`, `Op`, `Op.Author`, `Op.Hash`, `Op.Kind`, `Op.Lamport`, `Op.Name`, `Op.Person`, `Op.Prev`, `Op.Role`, `Op.RootURL`, `Op.Sig`, `Op.Subject`, `OpAdd`, `OpGenesis`, `OpKind`, `OpRemove`, `OpResume`, `OpSetRole`, `OpSuspend`, `Outbound`, `Outbound.Data`, `Outbound.To`, `ProjectID`, `ProjectID.String`, `Role`, `Role.String`, `Role.Valid`, `RoleAdmin`, `RoleReader`, `RoleWriter`, `Signer`, `Sync`, `Tick`.

- `protocol/vectors`: The embedded FS shares one canonical corpus with downstream conformance runners.
  `FS`.

## Licence

Licensed under the Mozilla Public License 2.0 (MPL-2.0). See [LICENSE](LICENSE).
