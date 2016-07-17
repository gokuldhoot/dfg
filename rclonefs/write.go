package main

import (
	"io"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
	"golang.org/x/net/context"
)

// WriteFileHandle is an open for write handle on a File
type WriteFileHandle struct {
	remote     string
	pipeReader *io.PipeReader
	pipeWriter *io.PipeWriter
	o          fs.Object
	result     chan error
	file       *File
}

// Check interface satisfied
var _ fusefs.Handle = (*WriteFileHandle)(nil)

func newWriteFileHandle(d *Dir, f *File, src fs.ObjectInfo) (*WriteFileHandle, error) {
	fh := &WriteFileHandle{
		remote: src.Remote(),
		result: make(chan error, 1),
		file:   f,
	}
	fh.pipeReader, fh.pipeWriter = io.Pipe()
	go func() {
		o, err := d.f.Put(fh.pipeReader, src)
		fh.o = o
		fh.result <- err
	}()
	return fh, nil
}

// Check interface satisfied
var _ fusefs.HandleWriter = (*WriteFileHandle)(nil)

// Write data to the file handle
func (fh *WriteFileHandle) Write(ctx context.Context, req *fuse.WriteRequest, resp *fuse.WriteResponse) error {
	fs.Debug(fh.remote, "WriteFileHandle.Write len=%d", len(req.Data))
	// FIXME should probably check the file isn't being seeked?
	n, err := fh.pipeWriter.Write(req.Data)
	resp.Size = n
	fh.file.written(int64(n))
	return err
}

// Check interface satisfied
var _ fusefs.HandleReleaser = (*WriteFileHandle)(nil)

// Release is called when we are finished with the file handle
func (fh *WriteFileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	fs.Debug(fh.remote, "WriteFileHandle.Release")
	writeCloseErr := fh.pipeWriter.Close()
	writeErr := <-fh.result
	fs.Debug(fh.remote, "WriteFileHandle.Release %v", writeErr)
	readCloseErr := fh.pipeReader.Close()
	if writeErr == nil {
		fh.file.setObject(fh.o)
	} else {
		fs.Debug(fh.remote, "WriteFileHandle.Release error: %v", writeErr)
		return writeErr
	}
	if writeCloseErr != nil {
		fs.Debug(fh.remote, "WriteFileHandle.Release error: %v", writeCloseErr)
		return writeCloseErr
	}
	fs.Debug(fh.remote, "WriteFileHandle.Release error: %v", readCloseErr)
	return readCloseErr
}
