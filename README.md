File Watcher
==============

This is a utility for watching a set of files in a directory,
then triggering a build and restarting a process when those files change.

It will recursively traverse the directories from the current working directory, and watch for changes in any
files that match the specified pattern.

It accepts three arguments on the command line.

* `-build-cmd` The command to rebuild your source.
* `-run-cmd` The command to run your process.
* `-file-pattern` The file pattern to monitor.  This only accepts simple patterns at the moment, like `*.go`, `manifest.json` etc
etc