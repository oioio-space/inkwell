"""Conservative source guard: reject launch API references, including values/aliases."""
import pathlib
import re
import sys
import unittest

ROOT = pathlib.Path(__file__).resolve().parent.parent
ALLOWED = {"internal/sidecar", "internal/sidecar/fetch"}
IMPORT = re.compile(r'(?:(\w+|\.)\s+)?"(os/exec|os|syscall|golang.org/x/sys/unix|C)"')


def launch_reference(text):
    for match in IMPORT.finditer(text):
        alias, package = match.groups()
        alias = (None if alias == "import" else alias) or package.rsplit("/", 1)[-1]
        if package == "os/exec":
            return True
        members = {
            "os": r"StartProcess",
            "syscall": r"Exec|ForkExec|StartProcess|SYS_EXECVE|SYS_EXECVEAT",
            "golang.org/x/sys/unix": r"Exec|ForkExec|StartProcess|SYS_EXECVE|SYS_EXECVEAT",
            "C": r"exec\w*|fexecve|system|popen|posix_spawn\w*|g_spawn\w*",
        }[package]
        prefix = r"(?<![\w.])" if alias == "." else r"\b" + re.escape(alias) + r"\s*\.\s*"
        if package == "C" and re.search(r"\b(?:" + members + r")\s*\(", text):
            return True
        if re.search(prefix + r"(?:" + members + r")\b", text):
            return True
    return False


def scan(root):
    failures = []
    paths = [root / "inkwell.go"]
    for folder in ("internal", "cmd", "protocol"):
        paths.extend((root / folder).rglob("*.go"))
    for path in paths:
        if path.name.endswith("_test.go") or not path.exists():
            continue
        name = path.relative_to(root).as_posix()
        parent = path.parent.relative_to(root).as_posix()
        text = path.read_text()
        if parent not in ALLOWED and launch_reference(text):
            failures.append(name + ": process launch outside allowlist")
        if parent == "internal/sidecar" and re.search(
                r'"/(?:usr/)?bin/|exec\.Command(?:Context)?\([^\n]*"(?:sh|setpriv|prlimit|unshare|mount|git|tar)"', text):
            failures.append(name + ": system executable in runtime runner")
    return failures


class BypassTests(unittest.TestCase):
    def test_bypass_fixtures(self):
        fixtures = [
            'import "syscall"; syscall.ForkExec(p,a,e)',
            'import "syscall"; syscall.StartProcess(p,a,e)',
            'import "syscall"; f := syscall.Exec; f(p,a,e)',
            'import s "syscall"; f := s.Exec',
            'import . "syscall"; ForkExec(p,a,e)',
            'import u "golang.org/x/sys/unix"; u.RawSyscall(u.SYS_EXECVE,0,0,0)',
            'import "syscall"; syscall.Syscall6(syscall.SYS_EXECVEAT,0,0,0,0,0,0)',
            'import o "os"; f := o.StartProcess',
            'import x "os/exec"; f := x.Command',
            'import "C"; C.execve(p,a,e)',
            'import "C"; C.system(p)',
            'import "C"; C.g_spawn_async(p)',
            '/* void wrapper() { system(p); } */ import "C"; C.wrapper()',
            '/* void wrapper() { g_spawn_sync(p); } */ import "C"; C.wrapper()',
        ]
        for fixture in fixtures:
            with self.subTest(fixture=fixture):
                self.assertTrue(launch_reference(fixture))
        self.assertFalse(launch_reference('import "os"; os.ReadFile(p)'))

    def test_root_fixture(self):
        import tempfile
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "inkwell.go").write_text('import s "syscall"; f := s.Exec')
            self.assertEqual(scan(root), ["inkwell.go: process launch outside allowlist"])


if __name__ == "__main__":
    if "--self-test" in sys.argv:
        unittest.main(argv=[sys.argv[0]])
    else:
        failures = scan(ROOT)
        print("\n".join(failures) if failures else "production exec allowlist: pass")
        sys.exit(bool(failures))
