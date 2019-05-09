lockfile
=========
Handle locking via pid files.

*Attention:* This is a fork of [Ingo Oeser's amazing work](https://github.com/nightlyone/lockfile)
whose behavior differs a bit. While the original package allows a process to
obtain the same lock twice, this fork forbids this behavior.

[![Build Status Unix][1]][2]
[![Build status Windows][3]][4]

[1]: https://secure.travis-ci.org/Acconut/lockfile.png
[2]: https://travis-ci.org/Acconut/lockfile
[3]: https://ci.appveyor.com/api/projects/status/bwy487h8cgue6up5?svg=true
[4]: https://ci.appveyor.com/project/Acconut/lockfile/branch/master



install
-------
Install [Go 1][5], either [from source][6] or [with a prepackaged binary][7].
For Windows support, Go 1.4 or newer is required.

Then run

	go get gopkg.in/Acconut/lockfile.v1

[5]: http://golang.org
[6]: http://golang.org/doc/install/source
[7]: http://golang.org/doc/install

LICENSE
-------
BSD

documentation
-------------
[package documentation at godoc.org](http://godoc.org/gopkg.in/Acconut/lockfile.v1)

contributing
============

Contributions are welcome. Please open an issue or send me a pull request for a dedicated branch.
Make sure the git commit hooks show it works.

git commit hooks
-----------------------
enable commit hooks via

        cd .git ; rm -rf hooks; ln -s ../git-hooks hooks ; cd ..
