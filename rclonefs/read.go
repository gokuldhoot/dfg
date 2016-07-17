package main

import (
	"io"

	"bazil.org/fuse"
	fusefs "bazil.org/fuse/fs"
	"github.com/ncw/rclone/fs"
	"golang.org/x/net/context"
)

// ReadFileHandle is an open for read file handle on a File
type ReadFileHandle struct {
	r io.ReadCloser
	o fs.Object
}

func newReadFileHandle(o fs.Object) (*ReadFileHandle, error) {
	r, err := o.Open()
	if err != nil {
		return nil, err
	}
	return &ReadFileHandle{r: r, o: o}, nil
}

// Check interface satisfied
var _ fusefs.Handle = (*ReadFileHandle)(nil)

// Check interface satisfied
var _ fusefs.HandleReleaser = (*ReadFileHandle)(nil)

// Release is called when the filehandle is finished with
func (fh *ReadFileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	fs.Debug(fh.o, "ReadFileHandle.Release")
	return fh.r.Close()
}

// Check interface satisfied
var _ fusefs.HandleReader = (*ReadFileHandle)(nil)

// Read from the file handle
func (fh *ReadFileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	fs.Debug(fh.o, "ReadFileHandle.Open")
	// We don't actually enforce Offset to match where previous read
	// ended. Maybe we should, but that would mean'd we need to track
	// it. The kernel *should* do it for us, based on the
	// fuse.OpenNonSeekable flag.
	//
	// One exception to the above is if we fail to fully populate a
	// page cache page; a read into page cache is always page aligned.
	// Make sure we never serve a partial read, to avoid that.
	buf := make([]byte, req.Size)
	n, err := io.ReadFull(fh.r, buf)
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}
	resp.Data = buf[:n]
	return err
}
