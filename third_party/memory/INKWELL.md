Source: github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 (MIT; see LICENSE).

Local replacement removes the macOS FreeMemory implementation's external
`vm_stat` command. FreeMemory reports unavailable (0) on macOS; libp2p only
uses TotalMemory, whose native sysctl implementation is preserved.
Other platform implementations are unchanged apart from gofmt/build tags.
