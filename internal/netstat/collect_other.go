//go:build !windows

package netstat

func platformSupported() bool { return false }

func defaultTableSource() tableSource { return unsupportedTableSource{} }

type unsupportedTableSource struct{}

func (unsupportedTableSource) Table(family string) ([]byte, error) {
	return nil, &TableError{Family: family, Code: 50}
}
