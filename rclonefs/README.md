# rclonefs - mount an rclone remote as a FUSE #

[![Logo](http://rclone.org/img/rclone-120x120.png)](http://rclone.org/)

rclonefs allows Linux and macOS to mount any of Rclone's cloud storage
systems as a file system with FUSE including

  * Google Drive
  * Amazon S3
  * Openstack Swift / Rackspace cloud files / Memset Memstore
  * Dropbox
  * Google Cloud Storage
  * Amazon Drive
  * Microsoft One Drive
  * Hubic
  * Backblaze B2
  * Yandex Disk
  * The local filesystem

Features

  * MD5/SHA1 hashes checked on upload and download for file integrity
  * Timestamps preserved on files
  * Server side rename (or copy/delete) where possible
  * Files stored as native objects
  * Directories cached in memory
  * Files not buffered in memory

***WARNING*** experimental!

## Usage ##

First set up your remote using `rclone config`.  Check it works with `rclone ls` etc.

Start the mount like this

    rclonefs remote:path/to/files /path/to/local/mount &

Stop the mount with

    fusermount -u /path/to/local/mount

Or with OS X

    umount -u /path/to/local/mount

rclonefs has some specific options which may help

    `-v`           - print debugging
    `--no-modtime` - don't read the modification time
    `--debug-fuse` - print lots of FUSE debugging (needs `-v`)

As well as lots of rclone's options.

## Limitations ##

This can only read files seqentially, or write files sequentially.  It
can't read and write or seek in files.

rclonefs inherits rclone's directory handling.  In rclone's world
directories don't really exist.  This means that empty directories
will have a tendency to disappear once they fall out of the directory
cache.

The bucket based FSes (eg swift, s3, google compute storage, b2) won't
work from the root - you will need to specify a bucket, or a path
within the bucket.  So `swift:` won't work whereas `swift:bucket` will
as will `swift:bucket/path`.

## Bugs ##

  * It has a lot of options from rclone which don't do anything
  * All the remotes should work for read, but some may not for write
    * those which need to know the size in advance won't - eg B2
    * maybe should pass in size as -1 to mean work it out

## TODO ##

  * Tests
  * Check hashes on upload/download
  * Preserve timestamps
  * Move directories
