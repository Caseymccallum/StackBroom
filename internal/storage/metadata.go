package storage

type fileMeta struct {
	Hidden         bool
	System         bool
	Reparse        bool
	Placeholder    bool
	Sparse         bool
	LinkCount      uint32
	FileID         string
	AllocatedBytes uint64
	AllocatedKnown bool
}
