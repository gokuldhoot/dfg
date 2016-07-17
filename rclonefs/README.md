# rclonefs - mount an rclone remote as a FUSE #

[![Logo](http://rclone.org/img/rclone-120x120.png)](http://rclone.org/)

This implements a FUSE mounting program for rclone.

***WARNING*** experimental!

First set up your remote using `rclone config`.  Check it works with `rclone ls` etc.

Start the mount like this

    rclonefs remote:path/to/files /path/to/local/mount &

Stop the mount with

    fusermount -u /path/to/local/mount

Or with OS X

    umount -u /path/to/local/mount

rclonefs has some specific options which may help

    --no-modtime - don't read the modification time
    --debug-fuse - print lots of FUSE debugging

As well as lots of rclone's options.

## Limitations ##

This can only read and write files sequentially.  So don't try running
a database on rclonefs!

rclonefs inherits rclone's directory handling.  In rclone's world
directories don't really exist.  This means that empty directories
will have a tendency to disappear once they fall out of the cache.

The bucket based FSes (eg swift, s3, google compute storage, b2) won't
work from the root - you will need to specify a bucket, or a path
within the bucket.  So `swift:` won't work whereas `swift:bucket` will
as will `swift:bucket/path`.

## Bugs ##

  * It has a lot of options from rclone which don't do anything
  * All the remotes should work for read, but some may not for write
    * those which need to know the size in advance won't - eg B2
    * maybe should pass in size as -1 to mean work it out
