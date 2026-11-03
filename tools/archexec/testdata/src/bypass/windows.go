package bypass

import w "golang.org/x/sys/windows"

var (
	createProcess       = w.CreateProcess       // want "forbidden launch"
	createProcessAsUser = w.CreateProcessAsUser // want "forbidden launch"
	shellExecute        = w.ShellExecute        // want "forbidden launch"
	diskSpace           = w.GetDiskFreeSpaceEx
)
