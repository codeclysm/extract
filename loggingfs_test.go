package extract_test

import (
	"fmt"
	"os"
)

// LoggingFS is a disk that logs every operation, useful for unit-testing.
type LoggingFS struct {
	Journal []*LoggedOp
}

// LoggedOp is an operation logged in a LoggingFS journal.
type LoggedOp struct {
	Op      string
	Path    string
	OldPath string
	Mode    os.FileMode
	Info    os.FileInfo
	Flags   int
	Err     error
}

func (op *LoggedOp) String() string {
	res := ""
	switch op.Op {
	case "link":
		res += fmt.Sprintf("link     %s -> %s", op.Path, op.OldPath)
	case "symlink":
		res += fmt.Sprintf("symlink  %s -> %s", op.Path, op.OldPath)
	case "mkdirall":
		res += fmt.Sprintf("mkdirall %v %s", op.Mode, op.Path)
	case "open":
		res += fmt.Sprintf("open     %v %s (flags=%04x)", op.Mode, op.Path, op.Flags)
	case "remove":
		res += fmt.Sprintf("remove   %v", op.Path)
	case "stat":
		res += fmt.Sprintf("stat     %v -> %v", op.Path, op.Info)
	case "chmod":
		res += fmt.Sprintf("chmod    %v %s", op.Mode, op.Path)
	default:
		panic("unknown LoggedOP " + op.Op)
	}
	if op.Err != nil {
		res += " error: " + op.Err.Error()
	} else {
		res += " success"
	}
	return res
}

func (m *LoggingFS) Link(oldname, newname string) error {
	err := os.Link(oldname, newname)
	op := &LoggedOp{
		Op:      "link",
		OldPath: oldname,
		Path:    newname,
		Err:     err,
	}
	m.Journal = append(m.Journal, op)
	fmt.Println("FS>", op)
	return err
}

func (m *LoggingFS) MkdirAll(path string, perm os.FileMode) error {
	err := os.MkdirAll(path, perm)
	op := &LoggedOp{
		Op:   "mkdirall",
		Path: path,
		Mode: perm,
		Err:  err,
	}
	m.Journal = append(m.Journal, op)
	fmt.Println("FS>", op)
	return err
}

func (m *LoggingFS) Symlink(oldname, newname string) error {
	err := os.Symlink(oldname, newname)
	op := &LoggedOp{
		Op:      "symlink",
		OldPath: oldname,
		Path:    newname,
		Err:     err,
	}
	m.Journal = append(m.Journal, op)
	fmt.Println("FS>", op)
	return err
}

func (m *LoggingFS) OpenFile(name string, flags int, perm os.FileMode) (*os.File, error) {
	f, err := os.OpenFile(name, flags, perm)
	op := &LoggedOp{
		Op:    "open",
		Path:  name,
		Mode:  perm,
		Flags: flags,
		Err:   err,
	}
	m.Journal = append(m.Journal, op)
	fmt.Println("FS>", op)
	return f, err
}

func (m *LoggingFS) Remove(path string) error {
	err := os.Remove(path)
	op := &LoggedOp{
		Op:   "remove",
		Path: path,
	}
	m.Journal = append(m.Journal, op)
	fmt.Println("FS>", op)
	return err
}

func (m *LoggingFS) Stat(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	op := &LoggedOp{
		Op:   "stat",
		Path: path,
		Info: info,
		Err:  err,
	}
	m.Journal = append(m.Journal, op)
	fmt.Println("FS>", op)
	return info, err
}

func (m *LoggingFS) Chmod(path string, mode os.FileMode) error {
	err := os.Chmod(path, mode)
	op := &LoggedOp{
		Op:   "chmod",
		Path: path,
		Mode: mode,
		Err:  err,
	}
	m.Journal = append(m.Journal, op)
	fmt.Println("FS>", op)
	return err
}

func (m *LoggingFS) String() string {
	res := ""
	for _, op := range m.Journal {
		res += op.String()
		res += "\n"
	}
	return res
}
