// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
	Package fmt implements formatted I/O with functions analogous
	to C's printf and scanf.  The format 'verbs' are derived from C's but
	are simpler.


	Printing

	The verbs:

	General:
		%v	the value in a default format.
			when printing structs, the plus flag (%+v) adds field names
		%#v	a Go-syntax representation of the value
		%T	a Go-syntax representation of the type of the value
		%%	a literal percent sign; consumes no value

	Boolean:
		%t	the word true or false
	Integer:
		%b	base 2
		%c	the character represented by the corresponding Unicode code point
		%d	base 10
		%o	base 8
		%q	a single-quoted character literal safely escaped with Go syntax.
		%x	base 16, with lower-case letters for a-f
		%X	base 16, with upper-case letters for A-F
		%U	Unicode format: U+1234; same as "U+%04X"
	Floating-point and complex constituents:
		%b	decimalless scientific notation with exponent a power of two,
			in the manner of strconv.FormatFloat with the 'b' format,
			e.g. -123456p-78
		%e	scientific notation, e.g. -1234.456e+78
		%E	scientific notation, e.g. -1234.456E+78
		%f	decimal point but no exponent, e.g. 123.456
		%F	synonym for %f
		%g	whichever of %e or %f produces more compact output
		%G	whichever of %E or %f produces more compact output
	String and slice of bytes:
		%s	the uninterpreted bytes of the string or slice
		%q	a double-quoted string safely escaped with Go syntax
		%x	base 16, lower-case, two characters per byte
		%X	base 16, upper-case, two characters per byte
	Pointer:
		%p	base 16 notation, with leading 0x

	There is no 'u' flag.  Integers are printed unsigned if they have unsigned type.
	Similarly, there is no need to specify the size of the operand (int8, int64).

	Width is specified by an optional decimal number immediately following the verb.
	If absent, the width is whatever is necessary to represent the value.
	Precision is specified after the (optional) width by a period followed by a
	decimal number. If no period is present, a default precision is used.
	A period with no following number specifies a precision of zero.
	Examples:
		%f:    default width, default precision
		%9f    width 9, default precision
		%.2f   default width, precision 2
		%9.2f  width 9, precision 2
		%9.f   width 9, precision 0

	Width and precision are measured in units of Unicode code points.
	(This differs from C's printf where the units are numbers
	of bytes.) Either or both of the flags may be replaced with the
	character '*', causing their values to be obtained from the next
	operand, which must be of type int.

	For most values, width is the minimum number of characters to output,
	padding the formatted form with spaces if necessary.
	For strings, precision is the maximum number of characters to output,
	truncating if necessary.

	For floating-point values, width sets the minimum width of the field and
	precision sets the number of places after the decimal, if appropriate,
	except that for %g/%G it sets the total number of digits. For example,
	given 123.45 the format %6.2f prints 123.45 while %.4g prints 123.5.
	The default precision for %e and %f is 6; for %g it is the smallest
	number of digits necessary to identify the value uniquely.

	For complex numbers, the width and precision apply to the two
	components independently and the result is parenthsized, so %f applied
	to 1.2+3.4i produces (1.200000+3.400000i).

	Other flags:
		+	always print a sign for numeric values;
			guarantee ASCII-only output for %q (%+q)
		-	pad with spaces on the right rather than the left (left-justify the field)
		#	alternate format: add leading 0 for octal (%#o), 0x for hex (%#x);
			0X for hex (%#X); suppress 0x for %p (%#p);
			for %q, print a raw (backquoted) string if strconv.CanBackquote
			returns true;
			write e.g. U+0078 'x' if the character is printable for %U (%#U).
		' '	(space) leave a space for elided sign in numbers (% d);
			put spaces between bytes printing strings or slices in hex (% x, % X)
		0	pad with leading zeros rather than spaces;
			for numbers, this moves the padding after the sign

	Flags are ignored by verbs that do not expect them.
	For example there is no alternate decimal format, so %#d and %d
	behave identically.

	For each Printf-like function, there is also a Print function
	that takes no format and is equivalent to saying %v for every
	operand.  Another variant Println inserts blanks between
	operands and appends a newline.

	Regardless of the verb, if an operand is an interface value,
	the internal concrete value is used, not the interface itself.
	Thus:
		var i interface{} = 23
		fmt.Printf("%v\n", i)
	will print 23.

	Except when printed using the the verbs %T and %p, special
	formatting considerations apply for operands that implement
	certain interfaces. In order of application:

	1. If an operand implements the Formatter interface, it will
	be invoked. Formatter provides fine control of formatting.

	2. If the %v verb is used with the # flag (%#v) and the operand
	implements the GoStringer interface, that will be invoked.

	If the format (which is implicitly %v for Println etc.) is valid
	for a string (%s %q %v %x %X), the following two rules apply:

	3. If an operand implements the error interface, the Error method
	will be invoked to convert the object to a string, which will then
	be formatted as required by the verb (if any).

	4. If an operand implements method String() string, that method
	will be invoked to convert the object to a string, which will then
	be formatted as required by the verb (if any).

	For compound operands such as slices and structs, the format
	applies to the elements of each operand, recursively, not to the
	operand as a whole. Thus %q will quote each element of a slice
	of strings, and %6.2f will control formatting for each element
	of a floating-point array.

	To avoid recursion in cases such as
		type X string
		func (x X) String() string { return Sprintf("<%s>", x) }
	convert the value before recurring:
		func (x X) String() string { return Sprintf("<%s>", string(x)) }

	Explicit argument indexes:

	In Printf, Sprintf, and Fprintf, the default behavior is for each
	formatting verb to format successive arguments passed in the call.
	However, the notation [n] immediately before the verb indicates that the
	nth one-indexed argument is to be formatted instead. The same notation
	before a '*' for a width or precision selects the argument index holding
	the value. After processing a bracketed expression [n], arguments n+1,
	n+2, etc. will be processed unless otherwise directed.

	For example,
		fmt.Sprintf("%[2]d %[1]d\n", 11, 22)
	will yield "22, 11", while
		fmt.Sprintf("%[3]*.[2]*[1]f", 12.0, 2, 6),
	equivalent to
		fmt.Sprintf("%6.2f", 12.0),
	will yield " 12.00". Because an explicit index affects subsequent verbs,
	this notation can be used to print the same values multiple times
	by resetting the index for the first argument to be repeated:
		fmt.Sprintf("%d %d %#[1]x %#x", 16, 17)
	will yield "16 17 0x10 0x11".

	Format errors:

	If an invalid argument is given for a verb, such as providing
	a string to %d, the generated string will contain a
	description of the problem, as in these examples:

		Wrong type or unknown verb: %!verb(type=value)
			Printf("%d", hi):          %!d(string=hi)
		Too many arguments: %!(EXTRA type=value)
			Printf("hi", "guys"):      hi%!(EXTRA string=guys)
		Too few arguments: %!verb(MISSING)
			Printf("hi%d"):            hi %!d(MISSING)
		Non-int for width or precision: %!(BADWIDTH) or %!(BADPREC)
			Printf("%*s", 4.5, "hi"):  %!(BADWIDTH)hi
			Printf("%.*s", 4.5, "hi"): %!(BADPREC)hi
		Invalid or invalid use of argument index: %!(BADINDEX)
			Printf("%*[2]d", 7):       %!d(BADINDEX)
			Printf("%.[2]d", 7):       %!d(BADINDEX)

	All errors begin with the string "%!" followed sometimes
	by a single character (the verb) and end with a parenthesized
	description.

	If an Error or String method triggers a panic when called by a
	print routine, the fmt package reformats the error message
	from the panic, decorating it with an indication that it came
	through the fmt package.  For example, if a String method
	calls panic("bad"), the resulting formatted message will look
	like
		%!s(PANIC=bad)

	The %!s just shows the print verb in use when the failure
	occurred.

	Scanning

	An analogous set of functions scans formatted text to yield
	values.  Scan, Scanf and Scanln read from os.Stdin; Fscan,
	Fscanf and Fscanln read from a specified io.Reader; Sscan,
	Sscanf and Sscanln read from an argument string.  Scanln,
	Fscanln and Sscanln stop scanning at a newline and require that
	the items be followed by one; Scanf, Fscanf and Sscanf require
	newlines in the input to match newlines in the format; the other
	routines treat newlines as spaces.

	Scanf, Fscanf, and Sscanf parse the arguments according to a
	format string, analogous to that of Printf.  For example, %x
	will scan an integer as a hexadecimal number, and %v will scan
	the default representation format for the value.

	The formats behave analogously to those of Printf with the
	following exceptions:

		%p is not implemented
		%T is not implemented
		%e %E %f %F %g %G are all equivalent and scan any floating point or complex value
		%s and %v on strings scan a space-delimited token
		Flags # and + are not implemented.

	The familiar base-setting prefixes 0 (octal) and 0x
	(hexadecimal) are accepted when scanning integers without a
	format or with the %v verb.

	Width is interpreted in the input text (%5s means at most
	five runes of input will be read to scan a string) but there
	is no syntax for scanning with a precision (no %5.2f, just
	%5f).

	When scanning with a format, all non-empty runs of space
	characters (except newline) are equivalent to a single
	space in both the format and the input.  With that proviso,
	text in the format string must match the input text; scanning
	stops if it does not, with the return value of the function
	indicating the number of arguments scanned.

	In all the scanning functions, a carriage return followed
	immediately by a newline is treated as a plain newline
	(\r\n means the same as \n).

	In all the scanning functions, if an operand implements method
	Scan (that is, it implements the Scanner interface) that
	method will be used to scan the text for that operand.  Also,
	if the number of arguments scanned is less than the number of
	arguments provided, an error is returned.

	All arguments to be scanned must be either pointers to basic
	types or implementations of the Scanner interface.

	Note: Fscan etc. can read one character (rune) past the input
	they return, which means that a loop calling a scan routine
	may skip some of the input.  This is usually a problem only
	when there is no space between input values.  If the reader
	provided to Fscan implements ReadRune, that method will be used
	to read characters.  If the reader also implements UnreadRune,
	that method will be used to save the character and successive
	calls will not lose data.  To attach ReadRune and UnreadRune
	methods to a reader without that capability, use
	bufio.NewReader.
*/

/*
	fmt 包实现了格式化I/O函数，类似于C的 printf 和 scanf.
	格式“占位符”衍生自C，但比C更简单。


	打印

	占位符：

	一般：
		%v	相应值的默认格式。在打印结构体时，“加号”标记（%+v）会添加字段名
		%#v	相应值的Go语法表示
		%T	相应值的类型的Go语法表示
		%%	字面上的百分号，并非值的占位符

	布尔：
		%t	单词 true 或 false。
	整数：
		%b	二进制表示
		%c	相应Unicode码点所表示的字符
		%d	十进制表示
		%o	八进制表示
		%q	单引号围绕的字符字面值，由Go语法安全地转义
		%x	十六进制表示，字母形式为小写 a-f
		%X	十六进制表示，字母形式为大写 A-F
		%U	Unicode格式：U+1234，等同于 "U+%04X"
	浮点数及其复合构成：
		%b	无小数部分的，指数为二的幂的科学计数法，与 strconv.FormatFloat
			的 'b' 转换格式一致。例如 -123456p-78
		%e	科学计数法，例如 -1234.456e+78
		%E	科学计数法，例如 -1234.456E+78
		%f	有小数点而无指数，例如 123.456
		%g	根据情况选择 %e 或 %f 以产生更紧凑的（无末尾的0）输出
		%G	根据情况选择 %E 或 %f 以产生更紧凑的（无末尾的0）输出
	字符串与字节切片：
		%s	字符串或切片的无解译字节
		%q	双引号围绕的字符串，由Go语法安全地转义
		%x	十六进制，小写字母，每字节两个字符
		%X	十六进制，大写字母，每字节两个字符
	指针：
		%p	十六进制表示，前缀 0x

	这里没有 'u' 标记。若整数为无符号类型，他们就会被打印成无符号的。类似地，
	这里也不需要指定操作数的大小（int8，int64）。

	宽度与精度的控制格式以Unicode码点为单位。（这点与C的 printf 不同，
	它以字节数为单位。）二者或其中之一均可用字符 '*' 表示，
	此时它们的值会从下一个操作数中获取，该操作数的类型必须为 int。

	对数值而言，宽度为该数值占用区域的最小宽度；精度为小数点之后的位数。
	但对于 %g/%G 而言，精度为所有数字的总数。例如，对于123.45，格式 %6.2f
	会打印123.45，而 %.4g 会打印123.5。%e 和 %f 的默认精度为6；但对于 %g 而言，
	它的默认精度为确定该值所必须的最小位数。

	对大多数值而言，宽度为输出的最小字符数，如果必要的话会为已格式化的形式填充空格。
	对字符串而言，精度为输出的最大字符数，如果必要的话会直接截断。

	其它标记：
		+	总打印数值的正负号；对于%q（%+q）保证只输出ASCII编码的字符。
		-	在右侧而非左侧填充空格（左对齐该区域）
		#	备用格式：为八进制添加前导 0（%#o），为十六进制添加前导 0x（%#x）或
			0X（%#X），为 %p（%#p）去掉前导 0x；对于 %q，若 strconv.CanBackquote
			返回 true，就会打印原始（即反引号围绕的）字符串；如果是可打印字符，
			%U（%#U）会写出该字符的Unicode编码形式（如字符 x 会被打印成 U+0078 'x'）。
		' '	（空格）为数值中省略的正负号留出空白（% d）；
			以十六进制（% x, % X）打印字符串或切片时，在字节之间用空格隔开
		0	填充前导的0而非空格；
			对于数字，这会将填充移到正负号之后

	标记有事会被占位符忽略，所以不要指望它们。例如十进制没有备用格式，因此 %#d
	与 %d 的行为相同。

	对于每一个 Printf 类的函数，都有一个 Print 函数，该函数不接受任何格式化，
	它等价于对每一个操作数都应用 %v。另一个变参函数 Println 会在操作数之间插入空白，
	并在末尾追加一个换行符。

	不考虑占位符的话，如果操作数是接口值，就会使用其内部的具体值，而非接口本身。
	因此：
		var i interface{} = 23
		fmt.Printf("%v\n", i)
	会打印 23。

	若一个操作数实现了 Formatter 接口，该接口就能更好地用于控制格式化。

	若其格式（它对于 Println 等函数是隐式的 %v）对于字符串是有效的
	（%s %q %v %x %X），以下两条规则也适用：

	1. 若一个操作数实现了 error 接口，Error 方法就能将该对象转换为字符串，
	随后会根据占位符的需要进行格式化。

	2. 若一个操作数实现了 String() string 方法，该方法能将该对象转换为字符串，
	随后会根据占位符的需要进行格式化。

	为避免以下这类递归的情况：
		type X string
		func (x X) String() string { return Sprintf("<%s>", x) }
	需要在递归前转换该值：
		func (x X) String() string { return Sprintf("<%s>", string(x)) }

	格式化错误：

	如果给占位符提供了无效的实参（例如将一个字符串提供给 %d），
	所生成的字符串会包含该问题的描述，如下例所示：

		类型错误或占位符未知：%!verb(type=value)
			Printf("%d", hi):          %!d(string=hi)
		实参太多：%!(EXTRA type=value)
			Printf("hi", "guys"):      hi%!(EXTRA string=guys)
		实参太少： %!verb(MISSING)
			Printf("hi%d"):            hi %!d(MISSING)
		宽度或精度不是int类型: %!(BADWIDTH) 或 %!(BADPREC)
			Printf("%*s", 4.5, "hi"):  %!(BADWIDTH)hi
			Printf("%.*s", 4.5, "hi"): %!(BADPREC)hi

	所有错误都始于“%!”，有时紧跟着单个字符（占位符），并以小括号括住的描述结尾。


	扫描

	一组类似的函数通过扫描已格式化的文本来产生值。Scan、Scanf 和 Scanln 从
	os.Stdin 中读取；Fscan、Fscanf 和 Fscanln 从指定的 io.Reader 中读取；
	Sscan、Sscanf 和 Sscanln 从实参字符串中读取。Scanln、Fscanln 和 Sscanln
	在换行符处停止扫描，且需要条目紧随换行符之后；Scanf、Fscanf 和 Sscanf
	需要输入换行符来匹配格式中的换行符；其它函数则将换行符视为空格。

	Scanf、Fscanf 和 Sscanf 根据格式字符串解析实参，类似于 Printf。例如，%x
	会将一个整数扫描为十六进制数，而 %v 则会扫描该值的默认表现格式。

	格式化行为类似于 Printf，但也有如下例外：

		%p 没有实现
		%T 没有实现
		%e %E %f %F %g %G 都完全等价，且可扫描任何浮点数或复合数值
		%s 和 %v 在扫描字符串时会将其中的空格作为分隔符
		标记 # 和 + 没有实现

	在或使用 %v 占位符扫描整数时，可接受友好的进制前缀0（八进制）和0x（十六进制）。

	宽度被解释为输入的文本（%5s 意为最多从输入中读取5个符文来扫描成字符串），
	而扫描函数则没有精度的语法（没有 %5.2f，只有 %5f）。

	当以某种格式进行扫描时，无论在格式中还是在输入中，所有非空的连续空白字符
	（除换行符外）都等价于单个空格。由于这种限制，格式字符串文本必须匹配输入的文本，
	如果不匹配，扫描过程就会停止，并返回已扫描的实参数。

	在所有的扫描参数中，若一个操作数实现了 Scan 方法（即它实现了 Scanner 接口），
	该操作数将使用该方法扫描其文本。此外，若已扫描的实参数少于所提供的实参数，
	就会返回一个错误。

	所有需要被扫描的实参都必须是基本类型或 Scanner 接口的实现。

	注意：Fscan 等函数会从输入中多读取一个字符（符文），因此，如果循环调用扫描函数，
	可能会跳过输入中的某些数据。一般只有在输入的数据中没有空白符时该问题才会出现。
	若提供给 Fscan 的读取器实现了 ReadRune，就会用该方法读取字符。若此读取器还实现了
	UnreadRune 方法，就会用该方法保存字符，而连续的调用将不会丢失数据。若要为没有
	ReadRune 和 UnreadRune 方法的读取器加上这些功能，需使用 bufio.NewReader。
*/
package fmt
