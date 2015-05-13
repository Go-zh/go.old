// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package mail implements parsing of mail messages.

For the most part, this package follows the syntax as specified by RFC 5322.
Notable divergences:
	* Obsolete address formats are not parsed, including addresses with
	  embedded route information.
	* Group addresses are not parsed.
	* The full range of spacing (the CFWS syntax element) is not supported,
	  such as breaking addresses across lines.
*/

/*
mail 包实现了解析邮件消息的功能.

大多数情况下，这个包遵循 RFC 5322 定义的格式。
需要注意的：
	* 过时的地址格式将不能被解析, 包括嵌入路由信息的地址格式。
	* 组地址不能被解析。
	* 全范围的空格（CFWS 语法元素）不支持，比如使用换行分隔地址。
*/
package mail

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/textproto"
	"strings"
	"time"
)

var debug = debugT(false)

type debugT bool

func (d debugT) Printf(format string, args ...interface{}) {
	if d {
		log.Printf(format, args...)
	}
}

// A Message represents a parsed mail message.

// Message代表解析后的邮件信息。
type Message struct {
	Header Header
	Body   io.Reader
}

// ReadMessage reads a message from r.
// The headers are parsed, and the body of the message will be available
// for reading from r.

// ReadMessage从r中读取一个邮件。
// 头部已经被解析了，而邮件体是可见的。
func ReadMessage(r io.Reader) (msg *Message, err error) {
	tp := textproto.NewReader(bufio.NewReader(r))

	hdr, err := tp.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}

	return &Message{
		Header: Header(hdr),
		Body:   tp.R,
	}, nil
}

// Layouts suitable for passing to time.Parse.
// These are tried in order.

// Layouts适合用来传递时间给time.Parse。
// 它们是按照顺序的排列的。
var dateLayouts []string

func init() {
	// Generate layouts based on RFC 5322, section 3.3.

	// 基于RFC 5322，3.3节，生成layouts。

	dows := [...]string{"", "Mon, "}   // day-of-week
	days := [...]string{"2", "02"}     // day = 1*2DIGIT
	years := [...]string{"2006", "06"} // year = 4*DIGIT / 2*DIGIT
	seconds := [...]string{":05", ""}  // second
	// "-0700 (MST)" is not in RFC 5322, but is common.
	zones := [...]string{"-0700", "MST", "-0700 (MST)"} // zone = (("+" / "-") 4DIGIT) / "GMT" / ...

	for _, dow := range dows {
		for _, day := range days {
			for _, year := range years {
				for _, second := range seconds {
					for _, zone := range zones {
						s := dow + day + " Jan " + year + " 15:04" + second + " " + zone
						dateLayouts = append(dateLayouts, s)
					}
				}
			}
		}
	}
}

func parseDate(date string) (time.Time, error) {
	for _, layout := range dateLayouts {
		t, err := time.Parse(layout, date)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("mail: header could not be parsed")
}

// A Header represents the key-value pairs in a mail message header.

// Header代表邮件header中的key-value值对。
type Header map[string][]string

// Get gets the first value associated with the given key.
// If there are no values associated with the key, Get returns "".

// Get获取根据key取出的第一个对应的值。
// 如果key没有对应的值，返回“”。
func (h Header) Get(key string) string {
	return textproto.MIMEHeader(h).Get(key)
}

var ErrHeaderNotPresent = errors.New("mail: header not in message")

// Date parses the Date header field.

// Date解析Date头部区域。
func (h Header) Date() (time.Time, error) {
	hdr := h.Get("Date")
	if hdr == "" {
		return time.Time{}, ErrHeaderNotPresent
	}
	return parseDate(hdr)
}

// AddressList parses the named header field as a list of addresses.

// AddressList将命名后的头部区域作为一列地址列表解析出来。
func (h Header) AddressList(key string) ([]*Address, error) {
	hdr := h.Get(key)
	if hdr == "" {
		return nil, ErrHeaderNotPresent
	}
	return ParseAddressList(hdr)
}

// Address represents a single mail address.
// An address such as "Barry Gibbs <bg@example.com>" is represented
// as Address{Name: "Barry Gibbs", Address: "bg@example.com"}.

// Address代表单个的邮件地址。
// 一个地址例如"Barry Gibbs <bg@example.com>"代表一个地址
// {Name: "Barry Gibbs", Address: "bg@example.com"}。
type Address struct {
	Name    string // Proper name; may be empty.
	Address string // user@domain
}

// Parses a single RFC 5322 address, e.g. "Barry Gibbs <bg@example.com>"

// 解析一个单独的RFC 5322地址，例如 “Barry Gibbs <bg@example.com>”
func ParseAddress(address string) (*Address, error) {
	return newAddrParser(address).parseAddress()
}

// ParseAddressList parses the given string as a list of addresses.

// ParseAddressList解析给的一列地址字符串
func ParseAddressList(list string) ([]*Address, error) {
	return newAddrParser(list).parseAddressList()
}

// String formats the address as a valid RFC 5322 address.
// If the address's name contains non-ASCII characters
// the name will be rendered according to RFC 2047.

// String格式化一个可视的RFC 5322地址。
// 如果地址名字包含非ASCII字符串，名字就会按照RFC 2047来解析。
func (a *Address) String() string {
	s := "<" + a.Address + ">"
	if a.Name == "" {
		return s
	}
	// If every character is printable ASCII, quoting is simple.
	allPrintable := true
	for i := 0; i < len(a.Name); i++ {
		// isWSP here should actually be isFWS,
		// but we don't support folding yet.
		if !isVchar(a.Name[i]) && !isWSP(a.Name[i]) {
			allPrintable = false
			break
		}
	}
	if allPrintable {
		b := bytes.NewBufferString(`"`)
		for i := 0; i < len(a.Name); i++ {
			if !isQtext(a.Name[i]) && !isWSP(a.Name[i]) {
				b.WriteByte('\\')
			}
			b.WriteByte(a.Name[i])
		}
		b.WriteString(`" `)
		b.WriteString(s)
		return b.String()
	}

	return mime.QEncoding.Encode("utf-8", a.Name) + " " + s
}

type addrParser []byte

func newAddrParser(s string) *addrParser {
	p := addrParser(s)
	return &p
}

func (p *addrParser) parseAddressList() ([]*Address, error) {
	var list []*Address
	for {
		p.skipSpace()
		addr, err := p.parseAddress()
		if err != nil {
			return nil, err
		}
		list = append(list, addr)

		p.skipSpace()
		if p.empty() {
			break
		}
		if !p.consume(',') {
			return nil, errors.New("mail: expected comma")
		}
	}
	return list, nil
}

// parseAddress parses a single RFC 5322 address at the start of p.

// parseAddress在p开始的时候解析单个RFC 5322地址。
func (p *addrParser) parseAddress() (addr *Address, err error) {
	debug.Printf("parseAddress: %q", *p)
	p.skipSpace()
	if p.empty() {
		return nil, errors.New("mail: no address")
	}

	// address = name-addr / addr-spec
	// TODO(dsymonds): Support parsing group address.

	// addr-spec has a more restricted grammar than name-addr,
	// so try parsing it first, and fallback to name-addr.
	// TODO(dsymonds): Is this really correct?
	spec, err := p.consumeAddrSpec()
	if err == nil {
		return &Address{
			Address: spec,
		}, err
	}
	debug.Printf("parseAddress: not an addr-spec: %v", err)
	debug.Printf("parseAddress: state is now %q", *p)

	// display-name
	var displayName string
	if p.peek() != '<' {
		displayName, err = p.consumePhrase()
		if err != nil {
			return nil, err
		}
	}
	debug.Printf("parseAddress: displayName=%q", displayName)

	// angle-addr = "<" addr-spec ">"
	p.skipSpace()
	if !p.consume('<') {
		return nil, errors.New("mail: no angle-addr")
	}
	spec, err = p.consumeAddrSpec()
	if err != nil {
		return nil, err
	}
	if !p.consume('>') {
		return nil, errors.New("mail: unclosed angle-addr")
	}
	debug.Printf("parseAddress: spec=%q", spec)

	return &Address{
		Name:    displayName,
		Address: spec,
	}, nil
}

// consumeAddrSpec parses a single RFC 5322 addr-spec at the start of p.

// consumeAddrSpec在p开始的时候解析单个RFC 5322 addr-spec。
func (p *addrParser) consumeAddrSpec() (spec string, err error) {
	debug.Printf("consumeAddrSpec: %q", *p)

	orig := *p
	defer func() {
		if err != nil {
			*p = orig
		}
	}()

	// local-part = dot-atom / quoted-string
	var localPart string
	p.skipSpace()
	if p.empty() {
		return "", errors.New("mail: no addr-spec")
	}
	if p.peek() == '"' {
		// quoted-string
		debug.Printf("consumeAddrSpec: parsing quoted-string")
		localPart, err = p.consumeQuotedString()
	} else {
		// dot-atom
		debug.Printf("consumeAddrSpec: parsing dot-atom")
		localPart, err = p.consumeAtom(true)
	}
	if err != nil {
		debug.Printf("consumeAddrSpec: failed: %v", err)
		return "", err
	}

	if !p.consume('@') {
		return "", errors.New("mail: missing @ in addr-spec")
	}

	// domain = dot-atom / domain-literal
	var domain string
	p.skipSpace()
	if p.empty() {
		return "", errors.New("mail: no domain in addr-spec")
	}
	// TODO(dsymonds): Handle domain-literal
	domain, err = p.consumeAtom(true)
	if err != nil {
		return "", err
	}

	return localPart + "@" + domain, nil
}

// consumePhrase parses the RFC 5322 phrase at the start of p.

// consumePhrase在p开始的时候解析RFC phrase。
func (p *addrParser) consumePhrase() (phrase string, err error) {
	debug.Printf("consumePhrase: [%s]", *p)
	// phrase = 1*word
	var words []string
	for {
		// word = atom / quoted-string
		var word string
		p.skipSpace()
		if p.empty() {
			return "", errors.New("mail: missing phrase")
		}
		if p.peek() == '"' {
			// quoted-string
			word, err = p.consumeQuotedString()
		} else {
			// atom
			// We actually parse dot-atom here to be more permissive
			// than what RFC 5322 specifies.
			word, err = p.consumeAtom(true)
		}

		if err == nil {
			word, err = decodeRFC2047Word(word)
		}

		if err != nil {
			break
		}
		debug.Printf("consumePhrase: consumed %q", word)
		words = append(words, word)
	}
	// Ignore any error if we got at least one word.
	if err != nil && len(words) == 0 {
		debug.Printf("consumePhrase: hit err: %v", err)
		return "", fmt.Errorf("mail: missing word in phrase: %v", err)
	}
	phrase = strings.Join(words, " ")
	return phrase, nil
}

// consumeQuotedString parses the quoted string at the start of p.

// consumeQuotedString在p开始的时候解析引用。
func (p *addrParser) consumeQuotedString() (qs string, err error) {
	// Assume first byte is '"'.
	i := 1
	qsb := make([]byte, 0, 10)
Loop:
	for {
		if i >= p.len() {
			return "", errors.New("mail: unclosed quoted-string")
		}
		switch c := (*p)[i]; {
		case c == '"':
			break Loop
		case c == '\\':
			if i+1 == p.len() {
				return "", errors.New("mail: unclosed quoted-string")
			}
			qsb = append(qsb, (*p)[i+1])
			i += 2
		case isQtext(c), c == ' ' || c == '\t':
			// qtext (printable US-ASCII excluding " and \), or
			// FWS (almost; we're ignoring CRLF)
			qsb = append(qsb, c)
			i++
		default:
			return "", fmt.Errorf("mail: bad character in quoted-string: %q", c)
		}
	}
	*p = (*p)[i+1:]
	return string(qsb), nil
}

// consumeAtom parses an RFC 5322 atom at the start of p.
// If dot is true, consumeAtom parses an RFC 5322 dot-atom instead.

// consumeAtom在p开始的时候解析RFC 5322原子操作。
// 如果有点的话，consumeAtom就按照RFC 5322解析。
func (p *addrParser) consumeAtom(dot bool) (atom string, err error) {
	if !isAtext(p.peek(), false) {
		return "", errors.New("mail: invalid string")
	}
	i := 1
	for ; i < p.len() && isAtext((*p)[i], dot); i++ {
	}
	atom, *p = string((*p)[:i]), (*p)[i:]
	return atom, nil
}

func (p *addrParser) consume(c byte) bool {
	if p.empty() || p.peek() != c {
		return false
	}
	*p = (*p)[1:]
	return true
}

// skipSpace skips the leading space and tab characters.

// skipSpace跳过开头的空格和tab字符。
func (p *addrParser) skipSpace() {
	*p = bytes.TrimLeft(*p, " \t")
}

func (p *addrParser) peek() byte {
	return (*p)[0]
}

func (p *addrParser) empty() bool {
	return p.len() == 0
}

func (p *addrParser) len() int {
	return len(*p)
}

func decodeRFC2047Word(s string) (string, error) {
	dec, err := rfc2047Decoder.Decode(s)
	if err == nil {
		return dec, nil
	}

	if _, ok := err.(charsetError); ok {
		return s, err
	}

	// Ignore invalid RFC 2047 encoded-word errors.
	return s, nil
}

var rfc2047Decoder = mime.WordDecoder{
	CharsetReader: func(charset string, input io.Reader) (io.Reader, error) {
		return nil, charsetError(charset)
	},
}

type charsetError string

func (e charsetError) Error() string {
	return fmt.Sprintf("charset not supported: %q", string(e))
}

var atextChars = []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"abcdefghijklmnopqrstuvwxyz" +
	"0123456789" +
	"!#$%&'*+-/=?^_`{|}~")

// isAtext reports whether c is an RFC 5322 atext character.
// If dot is true, period is included.

// isAtext当c是一个RFC 5322定义的atext字符的话返回true。
// 如果dot设置为true，就会考虑这个值。
func isAtext(c byte, dot bool) bool {
	if dot && c == '.' {
		return true
	}
	return bytes.IndexByte(atextChars, c) >= 0
}

// isQtext reports whether c is an RFC 5322 qtext character.

// isQtext 判断 c 是否为 RFC 5322 qtext 字符。
func isQtext(c byte) bool {
	// Printable US-ASCII, excluding backslash or quote.
	if c == '\\' || c == '"' {
		return false
	}
	return '!' <= c && c <= '~'
}

// isVchar reports whether c is an RFC 5322 VCHAR character.

// isVchar 判断 c 是否为 RFC 5322 VCHAR 字符。
func isVchar(c byte) bool {
	// Visible (printing) characters.
	return '!' <= c && c <= '~'
}

// isWSP reports whether c is a WSP (white space).
// WSP is a space or horizontal tab (RFC5234 Appendix B).

// isWSP 判断 c 是否为 WSP（空白字符）。
// WSP 是一个空格符或横向制表符（RFC5234 附录B）。
func isWSP(c byte) bool {
	return c == ' ' || c == '\t'
}
