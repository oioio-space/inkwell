package bypass

import "golang.org/x/sys/execabs" // want "forbidden import golang.org/x/sys/execabs"
var viaExecabs = execabs.Command
