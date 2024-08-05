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
	Flags   int
}

func (op *LoggedOp) String() string {
	switch op.Op {
	case "link":
		return fmt.Sprintf("link     %s -> %s", op.Path, op.OldPath)
	case "symlink":
		return fmt.Sprintf("symlink  %s -> %s", op.Path, op.OldPath)
	case "mkdirall":
		return fmt.Sprintf("mkdirall %v %s", op.Mode, op.Path)
	case "open":
		return fmt.Sprintf("open     %v %s (flags=%04x)", op.Mode, op.Path, op.Flags)
	case "remove":
		return fmt.Sprintf("remove   %v", op.Path)
	}
	panic("unknown LoggedOP " + op.Op)
}

func (m *LoggingFS) Link(oldname, newname string) error {
	m.Journal = append(m.Journal, &LoggedOp{
		Op:      "link",
		OldPath: oldname,
		Path:    newname,
	})
	return os.Link(oldname, newname)
}

func (m *LoggingFS) MkdirAll(path string, perm os.FileMode) error {
	m.Journal = append(m.Journal, &LoggedOp{
		Op:   "mkdirall",
		Path: path,
		Mode: perm,
	})
	return os.MkdirAll(path, perm)
}

func (m *LoggingFS) Symlink(oldname, newname string) error {
	m.Journal = append(m.Journal, &LoggedOp{
		Op:      "symlink",
		OldPath: oldname,
		Path:    newname,
	})
	return os.Symlink(oldname, newname)
}

func (m *LoggingFS) OpenFile(name string, flags int, perm os.FileMode) (*os.File, error) {
	m.Journal = append(m.Journal, &LoggedOp{
		Op:    "open",
		Path:  name,
		Mode:  perm,
		Flags: flags,
	})
	return os.OpenFile(os.DevNull, flags, perm)
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

func (m *LoggingFS) String() string {
	res := ""
	for _, op := range m.Journal {
		res += op.String()
		res += "\n"
	}
	return res
}
