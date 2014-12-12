# Go 编程语言

<!--
# The Go Programming Language

Go is an open source programming language that makes it easy to build simple,
reliable, and efficient software.

![Gopher image](doc/gopher/fiveyears.jpg)

For documentation about how to install and use Go,
visit https://golang.org/ or load doc/install-source.html
in your web browser.

Our canonical Git repository is located at https://go.googlesource.com/go.
(There is a mirror of the repository at https://github.com/golang/go.)

Please report issues here: https://golang.org/issue/new

Go is the work of hundreds of contributors. We appreciate your help!

To contribute, please read the contribution guidelines:
	https://golang.org/doc/contribute.html

Unless otherwise noted, the Go source files are distributed
under the BSD-style license found in the LICENSE file.
-->

Go 是一门开源的编程语言，它能让你轻松地构建简单、可靠而高效的软件。

![Gopher image](doc/gopher/fiveyears.jpg)

关于 Go 的安装和使用，请访问 https://golang.org/ ，或在你的浏览器中查看
`doc/install-source.html`

我们的 Git 代码库为 https://go.googlesource.com/go
（该代码库还有个镜像为 https://github.com/golang/go ）。

若发现问题请在此报告：https://golang.org/issue/new

Go 是数百名贡献者努力工作的结晶。我们感谢你们的帮助！

要做出贡献，请先阅读贡献指南：
	https://golang.org/doc/contribute.html

除特别注明外，Go 源码文件采用BSD风格授权协议分发。协议内容见 `LICENSE` 文件。

---

<!--
## Binary Distribution Notes

If you have just untarred a binary Go distribution, you need to set
the environment variable $GOROOT to the full path of the go
directory (the one containing this file).  You can omit the
variable if you unpack it into /usr/local/go, or if you rebuild
from sources by running all.bash (see doc/install-source.html).
You should also add the Go binary directory $GOROOT/bin
to your shell's path.

For example, if you extracted the tar file into $HOME/go, you might
put the following in your .profile:

	export GOROOT=$HOME/go
	export PATH=$PATH:$GOROOT/bin

See https://golang.org/doc/install or doc/install.html for more details.
-->

## 二进制分发注记

若你刚解包完 Go 的二进制分发包，那么还需要设置 `$GOROOT` 环境变量为该 go
目录（即包含本文件的目录）的完整路径。若你已将其解包到 `/usr/local/go`，或通过运行 `all.bash`
从源码重新构建了 Go（见 `doc/install-source.html` ），则可忽略此变量。此外，你还需要将 Go
的二进制文件目录 `$GOROOT/bin` 加入到你的 `$PATH` 环境变量中。

例如，若你将该 tar 文件提取到了 `$HOME/go` 中，那么还需在你的 `.profile` 中添加以下文本：

	export GOROOT=$HOME/go
	export PATH=$PATH:$GOROOT/bin

更多详情见 https://golang.org/doc/install 或 `doc/install.html`。
