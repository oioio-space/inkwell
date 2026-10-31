# Mesures architecture-audit (2026-10-31) — brut

## Versions
- go 1.27.3; mise 2026.6.11; golangci-lint 2.15.0 (mise.toml); gosec 2.30.0 (pinné mise); govulncheck (sec-scan.sh, 'Version: dev', Go go1.27.3)
- gocognit v1.2.0; gocyclo v0.6.0; dupl v1.0.0 (tags existants, pas de repli)

## 1. mise run lint — EXIT 0, aucun constat
Commande: mise run lint

## 1b. mise run scan:code — EXIT 0
gosec -quiet (sans sortie = 0 constat); govulncheck: 0 vulnérabilité appelée, 3 dans modules requis non appelés.

## 2. Couverture — mise run cover (heavy.sh) EXIT 0, total 90.4 %
Par paquet, profil fusionné coverage.out (blocs dédupliqués, coverpkg=./...) :
   1.1% github.com/oioio-space/inkwell/cmd/inkwell
  67.5% github.com/oioio-space/inkwell/cmd/sidecar-fetch
  77.7% github.com/oioio-space/inkwell/internal/transport/transporttest
  81.9% github.com/oioio-space/inkwell/cmd/inkwell-relay
  83.8% github.com/oioio-space/inkwell/internal/sidecar/fetch
  86.7% github.com/oioio-space/inkwell/internal/store
  87.1% github.com/oioio-space/inkwell/internal/blob
  87.3% github.com/oioio-space/inkwell/internal/invite
  87.9% github.com/oioio-space/inkwell/internal/project
  88.2% github.com/oioio-space/inkwell/internal/identity
  88.2% github.com/oioio-space/inkwell/internal/transport/p2p
  89.1% github.com/oioio-space/inkwell/internal/app
  91.1% github.com/oioio-space/inkwell/internal/typst
  92.7% github.com/oioio-space/inkwell/internal/sidecar
  95.8% github.com/oioio-space/inkwell/internal/transport/mux
  97.3% github.com/oioio-space/inkwell/internal/bridge
  97.6% github.com/oioio-space/inkwell/internal/trust
  98.7% github.com/oioio-space/inkwell/internal/transport/memnet
 100.0% github.com/oioio-space/inkwell/internal/transport
 100.0% github.com/oioio-space/inkwell/internal/transport/frame
 100.0% github.com/oioio-space/inkwell/internal/version
Sous 85 % : cmd/inkwell 1.1 ; cmd/sidecar-fetch 67.5 ; internal/transport/transporttest 77.7 ; cmd/inkwell-relay 81.9 ; internal/sidecar/fetch 83.8

Lignes 'coverage: X%' du log (par binaire de test, coverpkg=./...):
ok  	github.com/oioio-space/inkwell/cmd/inkwell	0.135s	coverage: 0.2% of statements in ./...
ok  	github.com/oioio-space/inkwell/cmd/inkwell-relay	0.033s	coverage: 1.3% of statements in ./...
ok  	github.com/oioio-space/inkwell/cmd/sidecar-fetch	0.016s	coverage: 0.6% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/app	25.954s	coverage: 57.3% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/blob	2.689s	coverage: 9.4% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/bridge	1.007s	coverage: 9.9% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/identity	0.232s	coverage: 1.4% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/invite	0.796s	coverage: 9.4% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/project	9.364s	coverage: 10.5% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/sidecar	4.539s	coverage: 3.4% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/sidecar/fetch	0.135s	coverage: 1.4% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/store	0.169s	coverage: 2.1% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/transport	0.012s	coverage: 0.1% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/transport/frame	0.274s	coverage: 1.2% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/transport/memnet	0.744s	coverage: 3.2% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/transport/mux	1.557s	coverage: 5.8% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/transport/p2p	8.849s	coverage: 12.7% of statements in ./...
	github.com/oioio-space/inkwell/internal/transport/transporttest		coverage: 0.0% of statements
ok  	github.com/oioio-space/inkwell/internal/trust	2.244s	coverage: 10.4% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/typst	21.639s	coverage: 12.3% of statements in ./...
ok  	github.com/oioio-space/inkwell/internal/version	0.015s	coverage: 0.2% of statements in ./...
Total coverage: 90.4%

## 3a. gocognit -over 15 internal (hors _test.go : 40 ; tests : 120)
280 app TestSuspend internal/app/suspend_test.go:74:1
147 trust (*refModel).fold internal/trust/reference_test.go:184:1
108 trust TestFoldInvariants internal/trust/reference_test.go:418:1
107 sidecar TestResolve internal/sidecar/store_test.go:114:1
87 trust TestSuspend internal/trust/suspend_test.go:31:1
81 project TestDecodeEncode internal/project/text_test.go:37:1
79 typst TestCompileHostile internal/typst/engine_test.go:471:1
68 bridge_test TestStoreVectors internal/bridge/storestream_test.go:86:1
67 sidecar TestSupervisor internal/sidecar/supervisor_test.go:106:1
64 blob_test TestResumeAfterCutDoesNotResendVerifiedGroups internal/blob/service_test.go:264:1
60 app runJoinerReceivesTheWholeTree internal/app/files_test.go:162:1
58 blob_test TestServerLimitsConcurrentRequests internal/blob/service_global_test.go:39:1
57 project (*Project).Watch internal/project/watch.go:27:1
56 blob_test TestForgedSliceRetriesOtherPeer internal/blob/service_test.go:498:1
56 typst TestPackageCache internal/typst/packages_test.go:73:1
55 p2p_test TestRelayThenDirect internal/transport/p2p/sim_test.go:305:1
52 mux_test FuzzSplitFrame internal/transport/mux/mux_test.go:150:1
52 trust (*sweep).members internal/trust/fold.go:416:1
50 app_test TestPreviewCompilesTheProject internal/app/preview_test.go:162:1
50 fetch TestFetch internal/sidecar/fetch/fetch_test.go:133:1
47 mux_test TestWireBoundaries internal/transport/mux/fragment_extra_test.go:307:1
45 p2p_test BenchmarkRepoLatencyDuringBulk internal/transport/p2p/bulk_test.go:258:1
44 sidecar TestRun internal/sidecar/run_test.go:17:1
43 app_test TestPreviewExport internal/app/preview_test.go:291:1
43 typst TestCompileDiagnostics internal/typst/engine_test.go:242:1
41 blob_test TestQuotas internal/blob/store_test.go:130:1
40 typst TestWatch internal/typst/watch_test.go:82:1
40 typst TestCompilePackages internal/typst/engine_test.go:353:1
39 bridge_test checkTreeVector internal/bridge/tree_test.go:52:1
39 blob_test TestServingSlotsAreReleased internal/blob/service_limits_test.go:73:1
39 trust (*sweep).decide internal/trust/fold.go:270:1
38 trust genDAG internal/trust/fold_test.go:18:1
38 sidecar TestManifest internal/sidecar/manifest_test.go:11:1
37 sidecar TestEmbeddedSidecarsRun internal/sidecar/embedded_test.go:32:1
37 trust TestSuspendAudit internal/trust/suspend_audit_test.go:40:1
36 trust TestCappedIssueKeepsSubjectOpsStanding internal/trust/regress2_test.go:77:1
35 typst (*Engine).Watch internal/typst/watch.go:32:1
35 typst readArchive internal/typst/packages.go:337:1
35 mux_test FuzzFragment internal/transport/mux/fragment_test.go:364:1
34 trust (*List).capPrev internal/trust/list.go:452:1
34 p2p_test TestRelayOnlyRefusesDirect internal/transport/p2p/sim_test.go:463:1
33 memnet_test BenchmarkSyncLatencyDuringBulk internal/transport/memnet/memnet_test.go:440:1
33 app_test TestPreviewScheduling internal/app/preview_test.go:455:1
33 bridge_test TestStoreCodecStrictness internal/bridge/codec_limits_test.go:565:1
31 typst TestAttachments internal/typst/attachments_test.go:46:1
31 p2p_test TestAdmitRefusesStranger internal/transport/p2p/node_test.go:254:1
31 blob TestDownloadNextAndCovers internal/blob/download_extra_test.go:84:1
30 p2p New internal/transport/p2p/node.go:270:1
30 mux (*muxer).run internal/transport/mux/mux.go:145:1
30 app TestPeople internal/app/people_test.go:35:1
30 app TestAttestationsBeforeRepoFrames internal/app/wiring_test.go:209:1
30 project FuzzValidateName internal/project/names_test.go:127:1
30 mux_test TestChannelIsolation internal/transport/mux/mux_test.go:67:1
29 project TestList internal/project/files_test.go:32:1
29 mux_test TestFragmentsSurviveFaults internal/transport/mux/fragment_test.go:223:1
28 typst TestFontCache internal/typst/fonts_test.go:12:1
28 identity_test TestWordlists2048Unique4Prefix internal/identity/fingerprint_test.go:61:1
28 mux_test TestPressureEvictsTheStalest internal/transport/mux/fragment_pressure_test.go:74:1
28 bridge_test TestEnvelopeVectors internal/bridge/envelope_test.go:38:1
27 p2p_test TestLoopbackSmoke internal/transport/p2p/node_test.go:176:1
27 app TestForgedBlobNeverVisible internal/app/files_test.go:472:1
27 blob (*Store).scan internal/blob/store.go:168:1
27 typst TestCompileFixtureOffline internal/typst/engine_test.go:78:1
27 bridge_test FuzzSmallCodecs internal/bridge/fuzz_small_test.go:14:1
27 typst TestPDFStrings internal/typst/unit_test.go:69:1
26 project TestPreClaimedPathNeverOverwritesTheUsersFile internal/project/owned_test.go:186:1
26 sidecar TestSupervisedTinymistLSP internal/sidecar/embedded_test.go:113:1
26 typst TestCompileInputs internal/typst/engine_test.go:678:1
26 typst TestBundledPackages internal/typst/packages_test.go:21:1
26 p2p_test TestBulkLoopback internal/transport/p2p/bulk_test.go:177:1
26 typst skipValue internal/typst/attachments.go:281:1
25 sidecar runHelper internal/sidecar/helper_test.go:59:1
25 typst TestCompileDeterministic internal/typst/engine_test.go:178:1
24 project (*Project).moveRenamed internal/project/tree.go:285:1
24 mux_test TestTypingDuringBigMessage internal/transport/mux/fragment_test.go:138:1
23 trust (*List).apply internal/trust/list.go:132:1
23 typst (*Engine).Compile internal/typst/engine.go:500:1
23 bridge_test TestEnvelopeLimitsAreExact internal/bridge/codec_limits_test.go:495:1
23 app TestRevokeDropsAndRefuses internal/app/wiring_test.go:162:1
23 trust (Op).validate internal/trust/op.go:191:1
23 mux_test TestBackgroundNotStarved internal/transport/mux/fragment_test.go:186:1
23 blob_test TestServingIsFairBetweenPeers internal/blob/service_test.go:863:1
22 project TestMigratesOwnedFilesOutOfTheManifest internal/project/owned_test.go:78:1
22 app TestSwitchProject internal/app/wiring_test.go:329:1
22 trust (*List).build internal/trust/list.go:375:1
22 app TestLanAskAndAnswer internal/app/invites_test.go:273:1
22 typst TestAnchors internal/typst/preview_test.go:58:1
22 blob_test TestConcurrentTransfers internal/blob/service_test.go:744:1
21 store (*KV).walk internal/store/kv.go:394:1
21 trust (*view).sweep internal/trust/fold.go:169:1
21 project TestApplyTreeProjectsEveryTextFile internal/project/tree_test.go:184:1
21 typst (*fontCache).Ensure internal/typst/fonts.go:75:1
21 bridge_test TestRouterForwardsBothWays internal/bridge/router_test.go:115:1
20 app TestAttestationsGossip internal/app/wiring_test.go:262:1
20 blob_test TestPoCPartialsBypassProjectQuota internal/blob/poc_test.go:19:1
20 blob Decode internal/blob/wire.go:168:1
20 mux_test TestInterleavedPeersAndChannels internal/transport/mux/fragment_extra_test.go:261:1
20 typst TestFailure internal/typst/unit_test.go:14:1
20 invite_test TestDeviceTicketKeepsPerson internal/invite/flow_test.go:305:1
20 typst (*Engine).packageView internal/typst/engine.go:391:1
20 app TestPendingHeadsHoldAWholeIndex internal/app/enforce_test.go:299:1
20 typst TestParseDiagnosticsLimitsAreExact internal/typst/bounds_extra_test.go:38:1
20 bridge_test FuzzDecodeFiles internal/bridge/storestream_test.go:623:1
20 blob (*Service).exchange internal/blob/service.go:557:1
20 mux_test TestSendBufferReuse internal/transport/mux/mux_extra_test.go:49:1
19 blob_test TestServerAnswersBusy internal/blob/service_test.go:674:1
19 blob TestHoldBoundaries internal/blob/download_extra_test.go:246:1
19 p2p (*Node).dialLoop internal/transport/p2p/dial.go:131:1
19 bridge_test TestRolesLimitsAreExact internal/bridge/codec_limits_test.go:433:1
19 bridge_test TestReattachReplacesConnAndReannounces internal/bridge/router_test.go:238:1
19 blob_test TestGC internal/blob/store_test.go:390:1
19 app TestRevokedWriterAttestationsAreForgotten internal/app/enforce_test.go:187:1
19 invite DecodeReply internal/invite/lobby.go:163:1
19 app (*Preview).compile internal/app/preview.go:283:1
19 bridge_test TestStoreRequestLimits internal/bridge/codec_limits_test.go:642:1
19 trust TestCreatorHandsOverToAnotherAdmin internal/trust/regress_test.go:183:1
19 project runWriteBlob internal/project/tree_test.go:236:1
19 fetch (*Config).download internal/sidecar/fetch/fetch.go:189:1
19 app TestNonMemberNeverReachesBridge internal/app/wiring_test.go:125:1
18 app (*invites).people internal/app/people.go:70:1
18 project TestApplyTreeMovesRenamedFiles internal/project/tree_test.go:326:1
18 project runApplyTreeRejectsHostilePaths internal/project/tree_test.go:41:1
18 mux_test TestPeerEventsOnEveryChannelInOrder internal/transport/mux/mux_extra_test.go:88:1
18 app (*Wiring).afterChange internal/app/wiring.go:410:1
18 typst DecodeDeck internal/typst/deck.go:58:1
18 app TestJoinByTicketText internal/app/invites_test.go:135:1
18 project (*Project).listDir internal/project/files.go:115:1
18 sidecar TestWithLimitsRoundsUp internal/sidecar/limits_extra_test.go:68:1
18 blob_test TestResumeWithUnusableMapStartsOver internal/blob/service_test.go:401:1
18 bridge_test TestReferencedAndSmallPayloads internal/bridge/codec_limits_test.go:323:1
17 bridge TestLimiterAllowsOncePerSecondPerReason internal/bridge/limiter_test.go:9:1
17 p2p_test latencies internal/transport/p2p/bulk_test.go:79:1
17 app runQuotasAtArrival internal/app/files_test.go:294:1
17 bridge DecodeTree internal/bridge/tree.go:110:1
17 project TestCheckTreeCollisions internal/project/boundaries_extra_test.go:277:1
17 trust TestSyncConverges internal/trust/sync_test.go:82:1
17 typst TestPageCount internal/typst/preview_test.go:10:1
17 trust TestIssueCapsPrev internal/trust/regress_test.go:413:1
17 p2p_test TestRelayOnlyGate internal/transport/p2p/sim_test.go:523:1
17 frame_test TestWriteProgressWire internal/transport/frame/progress_extra_test.go:56:1
17 trust TestSealIsOrderIndependent internal/trust/regress_test.go:458:1
17 mux_test TestBackpressureCount internal/transport/mux/mux_extra_test.go:241:1
17 frame_test TestWriteProgressDeadlines internal/transport/frame/progress_extra_test.go:91:1
17 invite decodeTicket internal/invite/ticket.go:148:1
16 p2p (*Node).Want internal/transport/p2p/dial.go:60:1
16 invite_test TestRedeemOnceExpiresWrongProject internal/invite/flow_test.go:26:1
16 store TestLoadRangeAndRemoveRange internal/store/kv_test.go:75:1
16 typst verifyTree internal/typst/packages.go:218:1
16 fetch TestFetchRefuses internal/sidecar/fetch/fetch_test.go:251:1
16 trust decodeMessage internal/trust/sync.go:179:1
16 p2p (*Node).handleStream internal/transport/p2p/stream.go:192:1
16 typst TestCompileConcurrencyCap internal/typst/limits_test.go:103:1
16 frame ReadReserve internal/transport/frame/frame.go:69:1
16 app TestAttestedPrecedesRepoFrame internal/app/enforce_test.go:68:1
16 project TestProjectionKeepsTheFileFormat internal/project/projection_test.go:29:1
16 p2p (*Bulk).handle internal/transport/p2p/bulk.go:127:1
16 typst (*Engine).readPages internal/typst/engine.go:651:1
16 project TestSyncPointKeptWithHash internal/project/syncpoint_test.go:12:1
16 blob (*Service).fetch internal/blob/service.go:462:1
16 typst New internal/typst/engine.go:187:1
exit status 1

## 3b. gocyclo -over 15 ./internal (hors _test.go : 25 ; tests : 61)
107 app TestSuspend internal/app/suspend_test.go:74:1
73 trust (*refModel).fold internal/trust/reference_test.go:184:1
57 project TestDecodeEncode internal/project/text_test.go:37:1
55 sidecar TestResolve internal/sidecar/store_test.go:114:1
52 trust TestFoldInvariants internal/trust/reference_test.go:418:1
46 trust TestSuspend internal/trust/suspend_test.go:31:1
40 sidecar TestSupervisor internal/sidecar/supervisor_test.go:106:1
39 bridge_test TestStoreVectors internal/bridge/storestream_test.go:86:1
37 typst TestCompileHostile internal/typst/engine_test.go:471:1
37 app_test TestPreviewCompilesTheProject internal/app/preview_test.go:162:1
37 app TestPeople internal/app/people_test.go:35:1
34 bridge_test TestStoreCodecStrictness internal/bridge/codec_limits_test.go:565:1
33 project (*Project).Watch internal/project/watch.go:27:1
33 app_test TestPreviewExport internal/app/preview_test.go:291:1
32 trust (Op).validate internal/trust/op.go:191:1
30 bridge_test checkTreeVector internal/bridge/tree_test.go:52:1
30 app runJoinerReceivesTheWholeTree internal/app/files_test.go:162:1
29 blob_test TestQuotas internal/blob/store_test.go:130:1
29 blob TestDownloadNextAndCovers internal/blob/download_extra_test.go:84:1
28 typst TestPackageCache internal/typst/packages_test.go:73:1
28 typst TestCompileDiagnostics internal/typst/engine_test.go:242:1
28 trust (*sweep).members internal/trust/fold.go:416:1
28 sidecar runHelper internal/sidecar/helper_test.go:59:1
27 trust (*List).build internal/trust/list.go:375:1
27 trust (*sweep).decide internal/trust/fold.go:270:1
26 trust (*List).capPrev internal/trust/list.go:452:1
26 sidecar TestRun internal/sidecar/run_test.go:17:1
26 fetch TestFetch internal/sidecar/fetch/fetch_test.go:133:1
25 typst TestCompilePackages internal/typst/engine_test.go:353:1
25 p2p New internal/transport/p2p/node.go:270:1
23 typst TestWatch internal/typst/watch_test.go:82:1
23 typst readArchive internal/typst/packages.go:337:1
23 typst TestParseDiagnosticsLimitsAreExact internal/typst/bounds_extra_test.go:38:1
23 app_test TestPreviewScheduling internal/app/preview_test.go:455:1
22 p2p_test TestRelayThenDirect internal/transport/p2p/sim_test.go:305:1
22 mux_test FuzzSplitFrame internal/transport/mux/mux_test.go:150:1
22 project TestMigratesOwnedFilesOutOfTheManifest internal/project/owned_test.go:78:1
22 bridge DecodeTree internal/bridge/tree.go:110:1
22 bridge (*StoreServer).handle internal/bridge/storestream.go:490:1
22 blob_test TestGC internal/blob/store_test.go:390:1
22 app TestLanAskAndAnswer internal/app/invites_test.go:273:1
21 typst TestAttachments internal/typst/attachments_test.go:46:1
21 sidecar TestEmbeddedSidecarsRun internal/sidecar/embedded_test.go:32:1
21 project FuzzValidateName internal/project/names_test.go:127:1
20 typst (*Engine).Watch internal/typst/watch.go:32:1
20 typst (*Engine).Compile internal/typst/engine.go:500:1
20 typst skipValue internal/typst/attachments.go:281:1
20 trust TestCappedIssueKeepsSubjectOpsStanding internal/trust/regress2_test.go:77:1
20 trust genDAG internal/trust/fold_test.go:18:1
20 project runWriteBlob internal/project/tree_test.go:236:1
20 bridge_test TestEnvelopeLimitsAreExact internal/bridge/codec_limits_test.go:495:1
19 typst DecodeDeck internal/typst/deck.go:58:1
19 invite_test TestRedeemOnceExpiresWrongProject internal/invite/flow_test.go:26:1
19 bridge (StoreRequest).validate internal/bridge/storestream.go:168:1
19 bridge_test TestStoreRequestLimits internal/bridge/codec_limits_test.go:642:1
19 bridge_test TestReferencedAndSmallPayloads internal/bridge/codec_limits_test.go:323:1
19 blob (*Store).scan internal/blob/store.go:168:1
19 blob_test TestForgedSliceRetriesOtherPeer internal/blob/service_test.go:498:1
19 blob TestHoldBoundaries internal/blob/download_extra_test.go:246:1
19 app TestJoinByTicketText internal/app/invites_test.go:135:1
18 typst TestAnchors internal/typst/preview_test.go:58:1
18 typst unescapeLiteral internal/typst/attachments.go:404:1
18 trust TestSuspendAudit internal/trust/suspend_audit_test.go:40:1
18 mux_test TestWireBoundaries internal/transport/mux/fragment_extra_test.go:307:1
18 mux (*muxer).reassemble internal/transport/mux/fragment.go:232:1
18 sidecar TestManifest internal/sidecar/manifest_test.go:11:1
18 project TestApplyTreeMovesRenamedFiles internal/project/tree_test.go:326:1
18 project TestApplyTreeProjectsEveryTextFile internal/project/tree_test.go:184:1
18 invite_test TestDeviceTicketKeepsPerson internal/invite/flow_test.go:305:1
18 blob_test TestResumeAfterCutDoesNotResendVerifiedGroups internal/blob/service_test.go:264:1
17 trust (*List).apply internal/trust/list.go:132:1
17 store TestLoadRangeAndRemoveRange internal/store/kv_test.go:75:1
17 fetch (*Config).download internal/sidecar/fetch/fetch.go:189:1
17 project (*Project).moveRenamed internal/project/tree.go:285:1
17 project TestSyncPointKeptWithHash internal/project/syncpoint_test.go:12:1
17 invite_test TestHelloOnlyWhileVisible internal/invite/flow_test.go:133:1
16 typst TestFontCache internal/typst/fonts_test.go:12:1
16 p2p_test TestRelayOnlyRefusesDirect internal/transport/p2p/sim_test.go:463:1
16 fetch extract internal/sidecar/fetch/fetch.go:256:1
16 sidecar TestSupervisedTinymistLSP internal/sidecar/embedded_test.go:113:1
16 project newText internal/project/text.go:240:1
16 project TestList internal/project/files_test.go:32:1
16 invite decodeTicket internal/invite/ticket.go:148:1
16 bridge_test TestEnvelopeVectors internal/bridge/envelope_test.go:38:1
16 blob Decode internal/blob/wire.go:168:1
16 blob_test TestPutGet internal/blob/store_test.go:18:1
exit status 1

## 4. dupl -t 100 ./internal — 3 groupes
internal/transport/memnet/memnet_test.go:304,314 <-> :316,326
internal/transport/p2p/bulk.go:162,183 <-> internal/transport/p2p/stream.go:230,251
internal/transport/p2p/bulk.go:100,121 <-> internal/transport/p2p/node.go:669,690

## 5. Règle de dépendance (go list -deps)
transport (hors p2p): transport, frame, memnet, mux, transporttest — aucune arête applicative
trust: transport — aucune arête applicative
identity: aucune arête
blob: store, transport — aucune arête applicative
libp2p importé par: internal/transport/p2p uniquement
automerge importé (Imports réels): aucun paquet Go (occurrences = chaînes d'URL 'automerge:' dans app/projects.go, bridge/tree_test.go)

## 6. Test qui impose la règle
_test.go: aucun 'go list -deps' / TestArchitecture : NON.
depguard dans .golangci.yml: absent : NON.
Règle appliquée par scripts/arch-check.sh (tâche mise 'arch', dans mise run ci) : PASS (exit 0).
