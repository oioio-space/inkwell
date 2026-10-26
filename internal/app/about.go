package app

import "github.com/oioio-space/inkwell/internal/version"

// AppVersion describes the running program (version, commit, build date), for
// the About dialog. It is the same data as "inkwell --version".
func (s *Session) AppVersion() version.Info { return version.Current() }
