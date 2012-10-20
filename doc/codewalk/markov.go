// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Generating random text: a Markov chain algorithm

Based on the program presented in the "Design and Implementation" chapter
of The Practice of Programming (Kernighan and Pike, Addison-Wesley 1999).
See also Computer Recreations, Scientific American 260, 122 - 125 (1989).

A Markov chain algorithm generates text by creating a statistical model of
potential textual suffixes for a given prefix. Consider this text:

	I am not a number! I am a free man!

Our Markov chain algorithm would arrange this text into this set of prefixes
and suffixes, or "chain": (This table assumes a prefix length of two words.)

	Prefix       Suffix

	"" ""        I
	"" I         am
	I am         a
	I am         not
	a free       man!
	am a         free
	am not       a
	a number!    I
	number! I    am
	not a        number!

To generate text using this table we select an initial prefix ("I am", for
example), choose one of the suffixes associated with that prefix at random
with probability determined by the input statistics ("a"),
and then create a new prefix by removing the first word from the prefix
and appending the suffix (making the new prefix is "am a"). Repeat this process
until we can't find any suffixes for the current prefix or we exceed the word
limit. (The word limit is necessary as the chain table may contain cycles.)

Our version of this program reads text from standard input, parsing it into a
Markov chain, and writes generated text to standard output.
The prefix and output lengths can be specified using the -prefix and -words
flags on the command-line.
*/

/*
生成随机文本：马尔可夫链算法

基于《程序设计实践》（Kernighan与Pike，Addison-Wesley 1994）的“设计与实现”
一章中提出的程序。

另请参阅《科学美国人》第260, 122 - 125 (1989)期《计算机娱乐》。

马尔科夫链算法通过创建一个统计模型来生成文本，该模型根据给定前缀潜在的文本后缀创建。
考虑以下文本：

	I am not a number! I am a free man!

我们的马尔可夫链算法会将这段文本整理成前缀和后缀的集合，或者说一个“链”：
（该表单假定一个前缀由两个单词组成。）

	前缀         后缀

	"" ""        I
	"" I         am
	I am         a
	I am         not
	a free       man!
	am a         free
	am not       a
	a number!    I
	number! I    am
	not a        number!

为了使用该表单生成文本，我们需要挑选一个初始前缀（比如说“I am”），并选择一个
与该前缀相关联的后缀，此后缀根据输入统计的概率随机决定（比如说“a”）；
接着通过从该前缀中移除第一个单词，并附加上该后缀来创建一个新的前缀（即让“am a”
作为新的前缀）。重复此过程，直到我们无法找到任何与当前前缀相关联后缀，或者超过了
单词的限制。（单词的限制是必须的，因为该链表可能包含周期。）

我们这个版本的程序从标准输入中读取，解析成一个马尔可夫链，然后将生成的文本写入
标准输出。前缀与输出长度可在命令行中使用 -prefix 以及 -words 标记来指定。
*/
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strings"
	"time"
)

// Prefix is a Markov chain prefix of one or more words.

// Prefix 为拥有一个或多个单词的链马尔可夫链的前缀。
type Prefix []string

// String returns the Prefix as a string (for use as a map key).

// String 将 Prefix 作为一个（用作映射键的）字符串返回。
func (p Prefix) String() string {
	return strings.Join(p, " ")
}

// Shift removes the first word from the Prefix and appends the given word.

// Shift 从 Prefix 中移除第一个单词并追加上给定的单词。
func (p Prefix) Shift(word string) {
	copy(p, p[1:])
	p[len(p)-1] = word
}

// Chain contains a map ("chain") of prefixes to a list of suffixes.
// A prefix is a string of prefixLen words joined with spaces.
// A suffix is a single word. A prefix can have multiple suffixes.

// Chain 包含一个从前缀到一个后缀列表的映射（“chain”）。
// 一个前缀就是一个加入了空格的，拥有 prefixLen 个单词的字符串。
// 一个后缀就是一个单词。一个前缀可拥有多个后缀。
type Chain struct {
	chain     map[string][]string
	prefixLen int
}

// NewChain returns a new Chain with prefixes of prefixLen words.

// NewChain 返回一个拥有 prefixLen 个单词前缀的 Chain。
func NewChain(prefixLen int) *Chain {
	return &Chain{make(map[string][]string), prefixLen}
}

// Build reads text from the provided Reader and
// parses it into prefixes and suffixes that are stored in Chain.

// Build 从提供的 Reader 中读取文本，并将它解析为存储了前缀和后缀的 Chain。
func (c *Chain) Build(r io.Reader) {
	br := bufio.NewReader(r)
	p := make(Prefix, c.prefixLen)
	for {
		var s string
		if _, err := fmt.Fscan(br, &s); err != nil {
			break
		}
		key := p.String()
		c.chain[key] = append(c.chain[key], s)
		p.Shift(s)
	}
}

// Generate returns a string of at most n words generated from Chain.

// Generate 返回一个从 Chain 生成的，最多有 n 个单词的字符串。
func (c *Chain) Generate(n int) string {
	p := make(Prefix, c.prefixLen)
	var words []string
	for i := 0; i < n; i++ {
		choices := c.chain[p.String()]
		if len(choices) == 0 {
			break
		}
		next := choices[rand.Intn(len(choices))]
		words = append(words, next)
		p.Shift(next)
	}
	return strings.Join(words, " ")
}

func main() {
	// 寄存命令行标记。
	numWords := flag.Int("words", 100, "maximum number of words to print")
	prefixLen := flag.Int("prefix", 2, "prefix length in words")

	flag.Parse()                     // 解析命令行标记。
	rand.Seed(time.Now().UnixNano()) // 设置随机数生成器的种子。

	c := NewChain(*prefixLen)     // 初始化一个新的 Chain。
	c.Build(os.Stdin)             // 从标准输入中构建链。
	text := c.Generate(*numWords) // 生成文本。
	fmt.Println(text)             // 将文本写入标准输出。
}

/*
func main() {
	// Register command-line flags.
	numWords := flag.Int("words", 100, "maximum number of words to print")
	prefixLen := flag.Int("prefix", 2, "prefix length in words")

	flag.Parse()                     // Parse command-line flags.
	rand.Seed(time.Now().UnixNano()) // Seed the random number generator.

	c := NewChain(*prefixLen)     // Initialize a new Chain.
	c.Build(os.Stdin)             // Build chains from standard input.
	text := c.Generate(*numWords) // Generate text.
	fmt.Println(text)             // Write text to standard output.
}
*/
