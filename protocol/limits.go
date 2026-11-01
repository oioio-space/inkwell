// SPDX-License-Identifier: MPL-2.0

package protocol

import "time"

// Version is the reference module release version.
const Version = "0.1.0"

const (
	// MaxConnectionsPerPeer is the maximum number of simultaneous connections with one peer, during a handover (§16, connections row).
	MaxConnectionsPerPeer = 2
	// MaxInboundConnections is the maximum number of inbound connections in total (§16, connections row).
	MaxInboundConnections = 128
	// MaxOutboundConnections is the maximum number of outbound connections in total (§16, connections row).
	MaxOutboundConnections = 128
	// MaxConnectedMembers is the number of members the connection totals are sized for (§16, connections row).
	MaxConnectedMembers = 64
	// ControlStreams is the maximum number of CONTROL streams per peer and direction (§16, streams row).
	ControlStreams = 1
	// RepoStreams is the maximum number of REPO streams per peer and direction (§16, streams row).
	RepoStreams = 1
	// ClosingRepoStreams is the extra REPO stream allowed per peer and direction while one is closing (§16, streams row).
	ClosingRepoStreams = 1
	// PresenceStreams is the maximum number of PRESENCE streams per peer and direction (§16, streams row).
	PresenceStreams = 1
	// UnreadLargeStreams is the maximum number of unread LARGE streams per peer and direction (§16, streams row).
	UnreadLargeStreams = 1
	// BulkStreams is the maximum number of bulk streams per peer and direction (§16, streams row).
	BulkStreams = 1
	// LobbyStreams is the maximum number of lobby streams per peer and direction (§16, streams row).
	LobbyStreams = 4
	// MaxSyncFrame is the maximum size of a sync frame, in bytes (§16, sync frame row).
	MaxSyncFrame = 98304
	// MaxBulkFrame is the maximum size of a bulk frame, in bytes (§16, bulk frame row).
	MaxBulkFrame = 1065005
	// MaxLobbyRequest is the maximum size of a lobby request, in bytes (§16, lobby row).
	MaxLobbyRequest = 4096
	// LobbyRequestTimeout is the time allowed to receive a lobby request (§16, lobby row).
	LobbyRequestTimeout = 10 * time.Second
	// MaxLobbyReply is the maximum size of a lobby reply, in bytes (§16, lobby row).
	MaxLobbyReply = 262144
	// LobbyProcessingTimeout is the time allowed to process a lobby request (§16, lobby row).
	LobbyProcessingTimeout = 3 * time.Minute
	// MaxHello is the maximum size of a HELLO message, in bytes (§16, HELLO row).
	MaxHello = 1024
	// HelloTimeout is the time allowed to complete the HELLO exchange (§16, HELLO row).
	HelloTimeout = 10 * time.Second
	// RepoInputBytes is the maximum size of one peer's REPO input queue, in bytes (§16, REPO input queue row).
	RepoInputBytes = 4 << 20
	// RepoInputFrames is the maximum number of frames in one peer's REPO input queue (§16, REPO input queue row).
	RepoInputFrames = 1024
	// ControlInputBytes is the maximum size of one peer's CONTROL input queue, in bytes (§16, CONTROL input queue row).
	ControlInputBytes = 512 << 10
	// MaxPendingResets is the maximum number of cancelled streams awaiting RESET_STREAM, per session (§16, cancelled streams row).
	MaxPendingResets = 1
	// ResetTimeout is the time a cancelled stream may wait for RESET_STREAM (§16, cancelled streams row).
	ResetTimeout = 10 * time.Second
	// IndivisibleSlotBytes is the disk slot reserved per connected peer for one pending indivisible change, in bytes (§16, pending indivisible changes row).
	IndivisibleSlotBytes = 16 << 20
	// IndivisibleRanksPerPeer is the maximum number of waiting ranks per peer for pending indivisible changes (§16, pending indivisible changes row).
	IndivisibleRanksPerPeer = 64
	// IndivisibleDiskBytes is the global disk cap for pending indivisible changes, absent peers included, in bytes (§16, pending indivisible changes row).
	IndivisibleDiskBytes = 1 << 30
	// IndivisibleIndexHistoryBytes is the stored history, in bytes, per entry of the indivisible-change index (§16, indivisible-change index row).
	IndivisibleIndexHistoryBytes = 4 << 20
	// MaxIndivisibleBytes is the maximum size of an indivisible announcement, in bytes (§16, indivisible announcement row).
	MaxIndivisibleBytes = 16777216
	// MaxLargeBytes is the maximum size of a LARGE stream object, in bytes (§16, LARGE stream row).
	MaxLargeBytes = 4194304
	// MaxPresenceEntries is the maximum number of PRESENCE entries per peer (§16, PRESENCE row).
	MaxPresenceEntries = 16
	// ControlOutputBytes is the maximum size of one peer's CONTROL output queue, in bytes (§16, CONTROL output queue row).
	ControlOutputBytes = 1 << 20
	// SetCursorsPerPeerSet is the maximum number of SET cursors per (peer, set) pair (§16, SET cursors row).
	SetCursorsPerPeerSet = 1
	// SetCursorsPerPeriod is the number of new SET cursors allowed per ReconcilePeriod (§16, SET cursors row).
	SetCursorsPerPeriod = 2
	// ReconcilePeriod is the period over which new SET cursors are counted (§16, SET cursors row).
	ReconcilePeriod = 15 * time.Second
	// AttestWantSentPerPeriod is the maximum number of ATTEST_WANT requests sent per peer and period (§16, ATTEST_WANT row).
	AttestWantSentPerPeriod = 64
	// AttestWantServedPerPeriod is the maximum number of ATTEST_WANT requests served per peer and period (§16, ATTEST_WANT row).
	AttestWantServedPerPeriod = 64
	// AttestWantTimeout is the time to wait for an ATTEST_WANT answer (§16, ATTEST_WANT row).
	AttestWantTimeout = 30 * time.Second
	// BridgeInlineBytes is the maximum size of one peer's repository output on the inline bridge path, in bytes (§16, repository output row).
	BridgeInlineBytes = 4 << 20
	// BridgeLargeObjects is the maximum number of objects on the LARGE bridge path per peer repository output (§16, repository output row).
	BridgeLargeObjects = 1
	// LargeAnnouncementsPerPeer is the maximum number of announced LARGE objects awaiting LARGE_REPLY per peer, sender side (§16, announced LARGE objects row).
	LargeAnnouncementsPerPeer = 1
	// LargeAnnouncementTimeout is the time an announced LARGE object may await LARGE_REPLY (§16, announced LARGE objects row).
	LargeAnnouncementTimeout = 10 * time.Minute
	// LargeReservationsPerPeer is the maximum number of reserved LARGE objects per peer, receiver side (§16, reserved LARGE objects row).
	LargeReservationsPerPeer = 1
	// MaxSessionsPerPeer is the maximum number of sessions per peer during a handover (§16, sessions row).
	MaxSessionsPerPeer = 2
	// QUICConnectionHeadroom is the headroom, in bytes, the QUIC connection window keeps above the stream window plus the initial stream window (§16, QUIC windows row).
	QUICConnectionHeadroom = 1 << 20
	// QUICMaxConnectionWindow is the maximum QUIC connection window, in bytes (§16, QUIC windows row).
	QUICMaxConnectionWindow = 15 << 20
	// QUICMaxStreamWindow is the maximum QUIC stream window, in bytes (§16, QUIC windows row).
	QUICMaxStreamWindow = 10 << 20
	// QUICInitialStreamWindow is the initial QUIC stream window, in bytes (§16, QUIC windows row).
	QUICInitialStreamWindow = 512 << 10
	// QUICInitialConnectionWindow is the initial QUIC connection window, in bytes (§16, QUIC windows row).
	QUICInitialConnectionWindow = 768 << 10
	// IgnorableFramesPerMinute is the maximum number of ignorable frames per peer and minute (§16, ignorable frames row).
	IgnorableFramesPerMinute = 256
	// IgnorableBytesPerMinute is the maximum size of ignorable frames per peer and minute, in bytes (§16, ignorable frames row).
	IgnorableBytesPerMinute = 256 << 10
	// LargeIdleTimeout is the time a reserved, read LARGE stream may go without a byte (§16, LARGE stream progress row).
	LargeIdleTimeout = 60 * time.Second
	// GenerationInitialBackoff is the initial per-peer backoff for generation commits and retries (§16, generation row).
	GenerationInitialBackoff = 2 * time.Second
	// GenerationMaxBackoff is the maximum per-peer backoff for generation commits and retries (§16, generation row).
	GenerationMaxBackoff = 60 * time.Second
	// GenerationBackoffMemory is how long the generation backoff state is remembered (§16, generation row).
	GenerationBackoffMemory = 10 * time.Minute
	// IncomingRepoStreamsPerPeriod is the maximum number of incoming REPO streams per peer within one initial backoff period (§16, generation row).
	IncomingRepoStreamsPerPeriod = 2
	// MaxGeneration is the maximum generation number, 2^32 - 1 (§16, generation row).
	MaxGeneration = 4294967295
	// BulkRequestsPerPeerHash is the maximum number of bulk requests per (peer, hash), requester side (§16, bulk requester row).
	BulkRequestsPerPeerHash = 1
	// BulkRequestsPerPeer is the maximum number of bulk requests per peer, requester side (§16, bulk requester row).
	BulkRequestsPerPeer = 2
	// BulkIdleTimeout is the time a bulk request may go without a byte, requester side (§16, bulk requester row).
	BulkIdleTimeout = 30 * time.Second
	// BulkRangeFailures is the maximum number of failures per range, requester side (§16, bulk requester row).
	BulkRangeFailures = 3
	// BulkConsecutiveBusy is the maximum number of consecutive BUSY answers, requester side (§16, bulk requester row).
	BulkConsecutiveBusy = 5
	// BulkServesPerPeer is the maximum number of bulk serves per peer, server side (§16, bulk server row).
	BulkServesPerPeer = 2
	// BulkMinGlobalServes is the floor of the total number of bulk serves, max(BulkMinGlobalServes, BulkGlobalServesPerPeer*n) (§16, bulk server row).
	BulkMinGlobalServes = 8
	// BulkGlobalServesPerPeer is the per-peer factor of the total number of bulk serves for n peers (§16, bulk server row).
	BulkGlobalServesPerPeer = 2
	// BulkBusyPerMinute is the maximum number of BUSY answers per peer and minute, server side (§16, bulk server row).
	BulkBusyPerMinute = 32
	// BulkFaultsBeforeBan is the number of faults after which a peer is banned from bulk serving (§16, bulk server row).
	BulkFaultsBeforeBan = 3
	// BulkBanDuration is the duration of the bulk-serving ban (§16, bulk server row).
	BulkBanDuration = 10 * time.Minute
	// ConcurrentBlobTransfers is the maximum number of concurrent blob transfers (§16, blob transfers row).
	ConcurrentBlobTransfers = 4
	// ReconnectInitialBackoff is the initial wait after a connection reset (§16, reset connection row).
	ReconnectInitialBackoff = 2 * time.Second
	// ReconnectMaxBackoff is the maximum wait after a connection reset (§16, reset connection row).
	ReconnectMaxBackoff = 60 * time.Second
	// ReconnectBackoffMemory is how long the reconnection backoff state is remembered (§16, reset connection row).
	ReconnectBackoffMemory = 10 * time.Minute
	// MaxMemberOps is the maximum number of operations in a sealed member list (§16, member list row).
	MaxMemberOps = 1024
	// MaxHaveHeads is the maximum number of heads in a member list `have` (§16, member list row).
	MaxHaveHeads = 256
	// MaxParentPendingObjects is the maximum number of objects awaiting their parents (§16, objects awaiting parents row).
	MaxParentPendingObjects = 256
	// ParentPendingPeriods is the number of periods an object may await its parents (§16, objects awaiting parents row).
	ParentPendingPeriods = 2
	// MaxDecisions is the maximum number of entries in the decision log (§16, decision log row).
	MaxDecisions = 8192
	// MaxDecisionsPerAuthor is the maximum number of decision log entries per author (§16, decision log row).
	MaxDecisionsPerAuthor = 1024
	// AnnouncementQuotaSlack is the number of announcements per author allowed above the quota Q* (§16, announcements row).
	AnnouncementQuotaSlack = 2
	// PendingAnnouncementsPerAuthor is the maximum number of announcements per author awaiting L2 (§16, announcements row).
	PendingAnnouncementsPerAuthor = 4
	// EvidencePerKeyEpoch is the maximum number of pieces of evidence per (key, epoch) (§16, evidence row).
	EvidencePerKeyEpoch = 1
	// EvidencePerKey is the maximum number of pieces of evidence per key (§16, evidence row).
	EvidencePerKey = 4
	// MaxOpBytes is the maximum encoded size of one trust operation, in bytes (§16, trust row).
	MaxOpBytes = 4096
	// MaxAttestationBytes is the maximum encoded size of one attestation, in bytes (§16, trust row).
	MaxAttestationBytes = 4096
	// MaxDecisionBytes is the maximum encoded size of one decision, in bytes (§16, trust row).
	MaxDecisionBytes = 8192
	// MaxAcknowledgmentBytes is the maximum encoded size of one acknowledgment, in bytes (§16, trust row).
	MaxAcknowledgmentBytes = 8192
	// MaxAnnouncementBytes is the maximum encoded size of one announcement, in bytes (§16, trust row).
	MaxAnnouncementBytes = 4096
	// MaxParents is the maximum number of parents of one operation (§16, trust row).
	MaxParents = 64
	// MaxHeads is the maximum number of heads in one message (§16, trust row).
	MaxHeads = 64
	// MaxEntries is the maximum number of entries in one message (§16, trust row).
	MaxEntries = 96
	// AttestationDistinctHeads is the maximum number of distinct heads kept per (document, signer) before eviction by lowest seq (§16, kept attestations row).
	AttestationDistinctHeads = 256
	// AttestationKeys is the minimum total number of (document, signer) keys of kept attestations (§16, kept attestations row).
	AttestationKeys = 65536
	// AttestationBytes is the minimum total size of kept attestations, in bytes (§16, kept attestations row).
	AttestationBytes = 64 << 20
	// UnknownSignerPendingPeriods is the number of periods an attestation may await its signer (§16, attestations awaiting signer row).
	UnknownSignerPendingPeriods = 2
	// HistoryRequestInterval is the minimum interval between history-request messages per (peer, document) (§16, noted refusals row).
	HistoryRequestInterval = 60 * time.Second
	// MaxTicketCharacters is the maximum length of a ticket, in characters (§16, ticket row).
	MaxTicketCharacters = 1024
	// TicketLifetime is the validity duration of a ticket (§16, ticket row).
	TicketLifetime = 15 * time.Minute
	// CBORMaxDepth is the maximum nesting depth of the CBOR profile (§16, CBOR profile row).
	CBORMaxDepth = 8
	// CBORMaxItems is the maximum number of items in one CBOR value (§16, CBOR profile row).
	CBORMaxItems = 4096
	// CBORMaxMap is the maximum number of entries of a CBOR map (§16, CBOR profile row).
	CBORMaxMap = 64
	// CBORMaxArray is the maximum number of elements of a CBOR array (§16, CBOR profile row).
	CBORMaxArray = 256
	// MaxExactInteger is the largest integer the CBOR profile accepts, 2^53 - 1 (§16, CBOR profile row).
	MaxExactInteger = 9007199254740991
	// MaxHandshakeBytes is the maximum size of a handshake message, in bytes (§16, handshake row).
	MaxHandshakeBytes = 1024
	// HandshakeBurst is the number of `arrive` messages accepted before rate limiting (§16, handshake row).
	HandshakeBurst = 3
	// HandshakeInterval is the interval between `arrive` messages once the burst is spent (§16, handshake row).
	HandshakeInterval = 20 * time.Second
	// MaxPeerIDBytes is the maximum length of a PeerID, in bytes (§16, identifiers row).
	MaxPeerIDBytes = 64
	// MaxDocumentIDBytes is the maximum length of a DocumentId, in bytes (§16, identifiers row).
	MaxDocumentIDBytes = 64
	// MaxProjectURLBytes is the maximum length of a `projectUrl`, in bytes (§16, identifiers row).
	MaxProjectURLBytes = 80
	// MaxEphemeralBytes is the maximum size of ephemeral `data`, in bytes (§16, ephemeral row).
	MaxEphemeralBytes = 1024
	// EphemeralBurst is the ephemeral message burst size (§16, ephemeral row).
	EphemeralBurst = 128
	// EphemeralPerSecond is the sustained ephemeral message rate, per second (§16, ephemeral row).
	EphemeralPerSecond = 64
	// DocumentPendingMessages is the maximum number of messages held per document that is not ready (§16, document not ready row).
	DocumentPendingMessages = 4
	// DocumentPendingBytesPerPeer is the maximum size of messages held per peer for documents not ready, in bytes (§16, document not ready row).
	DocumentPendingBytesPerPeer = 8 << 20
	// CoveragePendingMessages is the maximum number of messages awaiting coverage per document and peer (§16, awaiting coverage row).
	CoveragePendingMessages = 4
	// CoveragePendingBytesPerPeer is the maximum size of messages awaiting coverage per peer, in bytes (§16, awaiting coverage row).
	CoveragePendingBytesPerPeer = 8 << 20
	// CoveragePendingTimeout is the time a message may await coverage (§16, awaiting coverage row).
	CoveragePendingTimeout = 30 * time.Second
	// AuthorPendingMessages is the maximum number of messages awaiting an author tuple, per author (§16, awaiting author tuple row).
	AuthorPendingMessages = 4
	// AuthorPendingTimeout is the time a message may await an author tuple (§16, awaiting author tuple row).
	AuthorPendingTimeout = 30 * time.Second
	// CoverageHeads is the maximum number of heads per (document, signer) in coverage (§16, coverage row).
	CoverageHeads = 256
	// CoverageDocumentsPerSigner is the maximum number of documents per signer in coverage (§16, coverage row).
	CoverageDocumentsPerSigner = 1024
	// CoverageDocuments is the maximum total number of documents in coverage (§16, coverage row).
	CoverageDocuments = 10000
	// MaxChangeOperations is the maximum number of operations in one change (§16, guard row).
	MaxChangeOperations = 65536
	// MaxDecompressedChangeBytes is the maximum decompressed size of one change, in bytes (§16, guard row).
	MaxDecompressedChangeBytes = 16 << 20
	// GuardWorkerBytes is the memory cap of the guard Worker, in bytes (§16, guard row).
	GuardWorkerBytes = 256 << 20
	// PendingCreditMessagesPerDocument is the maximum number of messages awaiting credit per (peer, document) in a peer's output queue (§16, peer output queue row).
	PendingCreditMessagesPerDocument = 1
	// InitialSyncDocumentsPerPeer is the maximum number of documents in initial synchronization per peer (§16, peer output queue row).
	InitialSyncDocumentsPerPeer = 64
	// RepoOutputBytesPerPeer is the maximum size of one peer's REPO output queue, in bytes (§16, peer output queue row).
	RepoOutputBytesPerPeer = 16 << 20
	// MaxNetEnvelopeBytes is the maximum size of a `net` envelope, in bytes (§16, `net` envelope row).
	MaxNetEnvelopeBytes = 64 << 20
	// StoreLanes is the number of `store` lanes (§16, `store` row).
	StoreLanes = 1026
	// MaxStorePageBytes is the maximum size of a `store` page, in bytes (§16, `store` row).
	MaxStorePageBytes = 1 << 20
	// MaxStoreValueBytes is the maximum size of a `store` value, in bytes (§16, `store` row).
	MaxStoreValueBytes = 32 << 20
	// MaxStoreKeyComponents is the maximum number of components of a `store` key (§16, `store` row).
	MaxStoreKeyComponents = 4
	// MaxStoreKeyComponentBytes is the maximum size of one `store` key component, in bytes (§16, `store` row).
	MaxStoreKeyComponentBytes = 128
	// MaxPreviewRequestBytes is the maximum size of a `preview` request, in bytes (§16, `preview` row).
	MaxPreviewRequestBytes = 1 << 10
	// MaxPreviewPages is the maximum number of pages of a `preview` (§16, `preview` row).
	MaxPreviewPages = 40
	// MaxPreviewFrameBytes is the maximum size of a `preview` frame, in bytes (§16, `preview` row).
	MaxPreviewFrameBytes = 48 << 20
	// MaxPresentFrameBytes is the maximum size of a `present` frame, in bytes (§16, `present` row).
	MaxPresentFrameBytes = 8 << 20
	// MaxPresentWindows is the maximum number of `present` windows (§16, `present` row).
	MaxPresentWindows = 6
	// PresentFramesPerWindow is the maximum number of `present` frames per window (§16, `present` row).
	PresentFramesPerWindow = 256
	// PresentBytesPerWindow is the maximum size of `present` frames per window, in bytes (§16, `present` row).
	PresentBytesPerWindow = 32 << 20
)
