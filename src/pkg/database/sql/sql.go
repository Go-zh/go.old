// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sql provides a generic interface around SQL (or SQL-like)
// databases.
//
// The sql package must be used in conjunction with a database driver.
// See http://golang.org/s/sqldrivers for a list of drivers.
//
// For more usage examples, see the wiki page at
// http://golang.org/s/sqlwiki.

// sql 包提供了通用的SQL（或类SQL）数据库接口.
//
// sql 包必须与数据库驱动结合使用。驱动列表见 http://golang.org/s/sqldrivers。
//
// 更多使用范例见 http://golang.org/s/sqlwiki 的维基页面。
package sql

import (
	"container/list"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
)

var drivers = make(map[string]driver.Driver)

// Register makes a database driver available by the provided name.
// If Register is called twice with the same name or if driver is nil,
// it panics.

// Register使得数据库驱动可以使用事先定义好的名字使用。
// 如果使用同样的名字注册，或者是注册的的sql驱动是空的，Register会panic。
func Register(name string, driver driver.Driver) {
	if driver == nil {
		panic("sql: Register driver is nil")
	}
	if _, dup := drivers[name]; dup {
		panic("sql: Register called twice for driver " + name)
	}
	drivers[name] = driver
}

// RawBytes is a byte slice that holds a reference to memory owned by
// the database itself. After a Scan into a RawBytes, the slice is only
// valid until the next call to Next, Scan, or Close.

// RawBytes是一个字节数组，它是由数据库自己维护的一个内存空间。
// 当一个Scan被放入到RawBytes中之后，你下次调用Next，Scan或者Close就可以获取到slice了。
type RawBytes []byte

// NullString represents a string that may be null.
// NullString implements the Scanner interface so
// it can be used as a scan destination:
//
//  var s NullString
//  err := db.QueryRow("SELECT name FROM foo WHERE id=?", id).Scan(&s)
//  ...
//  if s.Valid {
//     // use s.String
//  } else {
//     // NULL value
//  }
//

// NullString代表一个可空的string。
// NUllString实现了Scanner接口，所以它可以被当做scan的目标变量使用:
//
//  var s NullString
//  err := db.QueryRow("SELECT name FROM foo WHERE id=?", id).Scan(&s)
//  ...
//  if s.Valid {
//     // use s.String
//  } else {
//     // NULL value
//  }
//
type NullString struct {
	String string
	Valid  bool // Valid is true if String is not NULL  // 如果String不是空，则Valid为true
}

// Scan implements the Scanner interface.

// Scan实现了Scanner接口。
func (ns *NullString) Scan(value interface{}) error {
	if value == nil {
		ns.String, ns.Valid = "", false
		return nil
	}
	ns.Valid = true
	return convertAssign(&ns.String, value)
}

// Value implements the driver Valuer interface.

// Value实现了driver Valuer接口。
func (ns NullString) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return ns.String, nil
}

// NullInt64 represents an int64 that may be null.
// NullInt64 implements the Scanner interface so
// it can be used as a scan destination, similar to NullString.

// NullInt64代表了可空的int64类型。
// NullInt64实现了Scanner接口，所以它和NullString一样可以被当做scan的目标变量。
type NullInt64 struct {
	Int64 int64
	Valid bool // Valid is true if Int64 is not NULL
}

// Scan implements the Scanner interface.

// Scan实现了Scaner接口。
func (n *NullInt64) Scan(value interface{}) error {
	if value == nil {
		n.Int64, n.Valid = 0, false
		return nil
	}
	n.Valid = true
	return convertAssign(&n.Int64, value)
}

// Value implements the driver Valuer interface.

// Value实现了driver Valuer接口。
func (n NullInt64) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Int64, nil
}

// NullFloat64 represents a float64 that may be null.
// NullFloat64 implements the Scanner interface so
// it can be used as a scan destination, similar to NullString.

// NullFloat64代表了可空的float64类型。
// NullFloat64实现了Scanner接口，所以它和NullString一样可以被当做scan的目标变量。
type NullFloat64 struct {
	Float64 float64
	Valid   bool // Valid is true if Float64 is not NULL  // 如果Float64非空，Valid就为true。
}

// Scan implements the Scanner interface.

// Scan实现了Scanner接口。
func (n *NullFloat64) Scan(value interface{}) error {
	if value == nil {
		n.Float64, n.Valid = 0, false
		return nil
	}
	n.Valid = true
	return convertAssign(&n.Float64, value)
}

// Value implements the driver Valuer interface.

// Value实现了driver的Valuer接口。
func (n NullFloat64) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Float64, nil
}

// NullBool represents a bool that may be null.
// NullBool implements the Scanner interface so
// it can be used as a scan destination, similar to NullString.

// NullBool代表了可空的bool类型。
// NullBool实现了Scanner接口，所以它和NullString一样可以被当做scan的目标变量。
type NullBool struct {
	Bool  bool
	Valid bool // Valid is true if Bool is not NULL  // 如果Bool非空，Valid就为true
}

// Scan implements the Scanner interface.

// Scan实现了Scanner接口。
func (n *NullBool) Scan(value interface{}) error {
	if value == nil {
		n.Bool, n.Valid = false, false
		return nil
	}
	n.Valid = true
	return convertAssign(&n.Bool, value)
}

// Value implements the driver Valuer interface.

// Value实现了driver的Valuer接口。
func (n NullBool) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Bool, nil
}

// Scanner is an interface used by Scan.

// Scanner是被Scan使用的接口。
type Scanner interface {
	// Scan assigns a value from a database driver.
	//
	// The src value will be of one of the following restricted
	// set of types:
	//
	//    int64
	//    float64
	//    bool
	//    []byte
	//    string
	//    time.Time
	//    nil - for NULL values
	//
	// An error should be returned if the value can not be stored
	// without loss of information.

	// Scan从数据库驱动中设置一个值。
	//
	// src值可以是下面限定的集中类型之一:
	//
	//    int64
	//    float64
	//    bool
	//    []byte
	//    string
	//    time.Time
	//    nil - for NULL values
	//
	// 如果数据只有通过丢失信息才能存储下来，这个方法就会返回error。
	Scan(src interface{}) error
}

// ErrNoRows is returned by Scan when QueryRow doesn't return a
// row. In such a case, QueryRow returns a placeholder *Row value that
// defers this error until a Scan.

// ErrNoRows是QueryRow的时候，当没有返回任何数据，Scan会返回的错误。
// 在这种情况下，QueryRow会返回一个*Row的标示符，直到调用Scan的时候才返回这个error。
var ErrNoRows = errors.New("sql: no rows in result set")

// DB is a database handle. It's safe for concurrent use by multiple
// goroutines.
//
// The sql package creates and frees connections automatically; it
// also maintains a free pool of idle connections. If the database has
// a concept of per-connection state, such state can only be reliably
// observed within a transaction. Once DB.Begin is called, the
// returned Tx is bound to a single connection. Once Commit or
// Rollback is called on the transaction, that transaction's
// connection is returned to DB's idle connection pool. The pool size
// can be controlled with SetMaxIdleConns.
// TODO：待译
type DB struct {
	driver driver.Driver
	dsn    string

	mu           sync.Mutex // protects following fields // 用于保护以下字段
	freeConn     *list.List // of *driverConn
	connRequests *list.List // of connRequest
	numOpen      int
	pendingOpens int
	// Used to signal the need for new connections
	// a goroutine running connectionOpener() reads on this chan and
	// maybeOpenNewConnections sends on the chan (one send per needed connection)
	// It is closed during db.Close(). The close tells the connectionOpener
	// goroutine to exit.
	openerCh chan struct{}
	closed   bool
	dep      map[finalCloser]depSet
	lastPut  map[*driverConn]string // stacktrace of last conn's put; debug only // conn 输出的最近一次栈跟踪；仅用于调试。
	maxIdle  int                    // zero means defaultMaxIdleConns; negative means 0 // 零表示 defaultMaxIdleConns；负值表示0
	maxOpen  int                    // <= 0 means unlimited
}

// driverConn wraps a driver.Conn with a mutex, to
// be held during all calls into the Conn. (including any calls onto
// interfaces returned via that Conn, such as calls on Tx, Stmt,
// Result, Rows)
// TODO：待译
type driverConn struct {
	db *DB

	sync.Mutex  // guards following
	ci          driver.Conn
	closed      bool
	finalClosed bool // ci.Close has been called
	openStmt    map[driver.Stmt]bool

	// guarded by db.mu
	inUse      bool
	onPut      []func() // code (with db.mu held) run when conn is next returned
	dbmuClosed bool     // same as closed, but guarded by db.mu, for connIfFree
	// This is the Element returned by db.freeConn.PushFront(conn).
	// It's used by connIfFree to remove the conn from the freeConn list.
	listElem *list.Element
}

func (dc *driverConn) releaseConn(err error) {
	dc.db.putConn(dc, err)
}

func (dc *driverConn) removeOpenStmt(si driver.Stmt) {
	dc.Lock()
	defer dc.Unlock()
	delete(dc.openStmt, si)
}

func (dc *driverConn) prepareLocked(query string) (driver.Stmt, error) {
	si, err := dc.ci.Prepare(query)
	if err == nil {
		// Track each driverConn's open statements, so we can close them
		// before closing the conn.
		//
		// TODO(bradfitz): let drivers opt out of caring about
		// stmt closes if the conn is about to close anyway? For now
		// do the safe thing, in case stmts need to be closed.
		//
		// TODO(bradfitz): after Go 1.2, closing driver.Stmts
		// should be moved to driverStmt, using unique
		// *driverStmts everywhere (including from
		// *Stmt.connStmt, instead of returning a
		// driver.Stmt), using driverStmt as a pointer
		// everywhere, and making it a finalCloser.
		if dc.openStmt == nil {
			dc.openStmt = make(map[driver.Stmt]bool)
		}
		dc.openStmt[si] = true
	}
	return si, err
}

// the dc.db's Mutex is held.
func (dc *driverConn) closeDBLocked() func() error {
	dc.Lock()
	defer dc.Unlock()
	if dc.closed {
		return func() error { return errors.New("sql: duplicate driverConn close") }
	}
	dc.closed = true
	return dc.db.removeDepLocked(dc, dc)
}

func (dc *driverConn) Close() error {
	dc.Lock()
	if dc.closed {
		dc.Unlock()
		return errors.New("sql: duplicate driverConn close")
	}
	dc.closed = true
	dc.Unlock() // not defer; removeDep finalClose calls may need to lock

	// And now updates that require holding dc.mu.Lock.
	dc.db.mu.Lock()
	dc.dbmuClosed = true
	fn := dc.db.removeDepLocked(dc, dc)
	dc.db.mu.Unlock()
	return fn()
}

func (dc *driverConn) finalClose() error {
	dc.Lock()

	for si := range dc.openStmt {
		si.Close()
	}
	dc.openStmt = nil

	err := dc.ci.Close()
	dc.ci = nil
	dc.finalClosed = true
	dc.Unlock()

	dc.db.mu.Lock()
	dc.db.numOpen--
	dc.db.maybeOpenNewConnections()
	dc.db.mu.Unlock()

	return err
}

// driverStmt associates a driver.Stmt with the
// *driverConn from which it came, so the driverConn's lock can be
// held during calls.
// TODO：待译
type driverStmt struct {
	sync.Locker // the *driverConn
	si          driver.Stmt
}

func (ds *driverStmt) Close() error {
	ds.Lock()
	defer ds.Unlock()
	return ds.si.Close()
}

// depSet is a finalCloser's outstanding dependencies
// TODO：待译
type depSet map[interface{}]bool // set of true bools

// The finalCloser interface is used by (*DB).addDep and related
// dependency reference counting.
// TODO：待译
type finalCloser interface {
	// finalClose is called when the reference count of an object
	// goes to zero. (*DB).mu is not held while calling it.
	finalClose() error
}

// addDep notes that x now depends on dep, and x's finalClose won't be
// called until all of x's dependencies are removed with removeDep.
// TODO：待译
func (db *DB) addDep(x finalCloser, dep interface{}) {
	//println(fmt.Sprintf("addDep(%T %p, %T %p)", x, x, dep, dep))
	db.mu.Lock()
	defer db.mu.Unlock()
	db.addDepLocked(x, dep)
}

func (db *DB) addDepLocked(x finalCloser, dep interface{}) {
	if db.dep == nil {
		db.dep = make(map[finalCloser]depSet)
	}
	xdep := db.dep[x]
	if xdep == nil {
		xdep = make(depSet)
		db.dep[x] = xdep
	}
	xdep[dep] = true
}

// removeDep notes that x no longer depends on dep.
// If x still has dependencies, nil is returned.
// If x no longer has any dependencies, its finalClose method will be
// called and its error value will be returned.
// TODO：待译
func (db *DB) removeDep(x finalCloser, dep interface{}) error {
	db.mu.Lock()
	fn := db.removeDepLocked(x, dep)
	db.mu.Unlock()
	return fn()
}

func (db *DB) removeDepLocked(x finalCloser, dep interface{}) func() error {
	//println(fmt.Sprintf("removeDep(%T %p, %T %p)", x, x, dep, dep))

	xdep, ok := db.dep[x]
	if !ok {
		panic(fmt.Sprintf("unpaired removeDep: no deps for %T", x))
	}

	l0 := len(xdep)
	delete(xdep, dep)

	switch len(xdep) {
	case l0:
		// Nothing removed. Shouldn't happen.
		panic(fmt.Sprintf("unpaired removeDep: no %T dep on %T", dep, x))
	case 0:
		// No more dependencies.
		delete(db.dep, x)
		return x.finalClose
	default:
		// Dependencies remain.
		return func() error { return nil }
	}
}

// This is the size of the connectionOpener request chan (dn.openerCh).
// This value should be larger than the maximum typical value
// used for db.maxOpen. If maxOpen is significantly larger than
// connectionRequestQueueSize then it is possible for ALL calls into the *DB
// to block until the connectionOpener can satify the backlog of requests.
var connectionRequestQueueSize = 1000000

// Open opens a database specified by its database driver name and a
// driver-specific data source name, usually consisting of at least a
// database name and connection information.
//
// Most users will open a database via a driver-specific connection
// helper function that returns a *DB. No database drivers are included
// in the Go standard library. See http://golang.org/s/sqldrivers for
// a list of third-party drivers.
//
// Open may just validate its arguments without creating a connection
// to the database. To verify that the data source name is valid, call
// Ping.

// Open打开一个数据库，这个数据库是由其驱动名称和驱动制定的数据源信息打开的，这个数据源信息通常
// 是由至少一个数据库名字和连接信息组成的。
//
// 多数用户通过指定的驱动连接辅助函数来打开一个数据库。打开数据库之后会返回*DB。
//
// TODO：待译
func Open(driverName, dataSourceName string) (*DB, error) {
	driveri, ok := drivers[driverName]
	if !ok {
		return nil, fmt.Errorf("sql: unknown driver %q (forgotten import?)", driverName)
	}
	db := &DB{
		driver:   driveri,
		dsn:      dataSourceName,
		openerCh: make(chan struct{}, connectionRequestQueueSize),
		lastPut:  make(map[*driverConn]string),
	}
	db.freeConn = list.New()
	db.connRequests = list.New()
	go db.connectionOpener()
	return db, nil
}

// Ping verifies a connection to the database is still alive,
// establishing a connection if necessary.
// TODO：待译
func (db *DB) Ping() error {
	// TODO(bradfitz): give drivers an optional hook to implement
	// this in a more efficient or more reliable way, if they
	// have one.
	dc, err := db.conn()
	if err != nil {
		return err
	}
	db.putConn(dc, nil)
	return nil
}

// Close closes the database, releasing any open resources.

// Close关闭数据库，释放一些使用中的资源。
func (db *DB) Close() error {
	db.mu.Lock()
	if db.closed { // Make DB.Close idempotent
		db.mu.Unlock()
		return nil
	}
	close(db.openerCh)
	var err error
	fns := make([]func() error, 0, db.freeConn.Len())
	for db.freeConn.Front() != nil {
		dc := db.freeConn.Front().Value.(*driverConn)
		dc.listElem = nil
		fns = append(fns, dc.closeDBLocked())
		db.freeConn.Remove(db.freeConn.Front())
	}
	db.closed = true
	for db.connRequests.Front() != nil {
		req := db.connRequests.Front().Value.(connRequest)
		db.connRequests.Remove(db.connRequests.Front())
		close(req)
	}
	db.mu.Unlock()
	for _, fn := range fns {
		err1 := fn()
		if err1 != nil {
			err = err1
		}
	}
	return err
}

const defaultMaxIdleConns = 2

func (db *DB) maxIdleConnsLocked() int {
	n := db.maxIdle
	switch {
	case n == 0:
		// TODO(bradfitz): ask driver, if supported, for its default preference
		return defaultMaxIdleConns
	case n < 0:
		return 0
	default:
		return n
	}
}

// SetMaxIdleConns sets the maximum number of connections in the idle
// connection pool.
//
// If MaxOpenConns is greater than 0 but less than the new MaxIdleConns
// then the new MaxIdleConns will be reduced to match the MaxOpenConns limit
//
// If n <= 0, no idle connections are retained.
// TODO：待译
func (db *DB) SetMaxIdleConns(n int) {
	db.mu.Lock()
	if n > 0 {
		db.maxIdle = n
	} else {
		// No idle connections.
		db.maxIdle = -1
	}
	// Make sure maxIdle doesn't exceed maxOpen
	if db.maxOpen > 0 && db.maxIdleConnsLocked() > db.maxOpen {
		db.maxIdle = db.maxOpen
	}
	var closing []*driverConn
	for db.freeConn.Len() > db.maxIdleConnsLocked() {
		dc := db.freeConn.Back().Value.(*driverConn)
		dc.listElem = nil
		db.freeConn.Remove(db.freeConn.Back())
		closing = append(closing, dc)
	}
	db.mu.Unlock()
	for _, c := range closing {
		c.Close()
	}
}

// SetMaxOpenConns sets the maximum number of open connections to the database.
//
// If MaxIdleConns is greater than 0 and the new MaxOpenConns is less than
// MaxIdleConns, then MaxIdleConns will be reduced to match the new
// MaxOpenConns limit
//
// If n <= 0, then there is no limit on the number of open connections.
// The default is 0 (unlimited).
func (db *DB) SetMaxOpenConns(n int) {
	db.mu.Lock()
	db.maxOpen = n
	if n < 0 {
		db.maxOpen = 0
	}
	syncMaxIdle := db.maxOpen > 0 && db.maxIdleConnsLocked() > db.maxOpen
	db.mu.Unlock()
	if syncMaxIdle {
		db.SetMaxIdleConns(n)
	}
}

// Assumes db.mu is locked.
// If there are connRequests and the connection limit hasn't been reached,
// then tell the connectionOpener to open new connections.
func (db *DB) maybeOpenNewConnections() {
	numRequests := db.connRequests.Len() - db.pendingOpens
	if db.maxOpen > 0 {
		numCanOpen := db.maxOpen - (db.numOpen + db.pendingOpens)
		if numRequests > numCanOpen {
			numRequests = numCanOpen
		}
	}
	for numRequests > 0 {
		db.pendingOpens++
		numRequests--
		db.openerCh <- struct{}{}
	}
}

// Runs in a separate goroutine, opens new connections when requested.
func (db *DB) connectionOpener() {
	for _ = range db.openerCh {
		db.openNewConnection()
	}
}

// Open one new connection
func (db *DB) openNewConnection() {
	ci, err := db.driver.Open(db.dsn)
	db.mu.Lock()
	defer db.mu.Unlock()
	if db.closed {
		if err == nil {
			ci.Close()
		}
		return
	}
	db.pendingOpens--
	if err != nil {
		db.putConnDBLocked(nil, err)
		return
	}
	dc := &driverConn{
		db: db,
		ci: ci,
	}
	if db.putConnDBLocked(dc, err) {
		db.addDepLocked(dc, dc)
		db.numOpen++
	} else {
		ci.Close()
	}
}

// connRequest represents one request for a new connection
// When there are no idle connections available, DB.conn will create
// a new connRequest and put it on the db.connRequests list.
type connRequest chan<- interface{} // takes either a *driverConn or an error

var errDBClosed = errors.New("sql: database is closed")

// conn returns a newly-opened or cached *driverConn

// conn返回新创建的，或者是缓存住的*driverConn。
func (db *DB) conn() (*driverConn, error) {
	db.mu.Lock()
	if db.closed {
		db.mu.Unlock()
		return nil, errDBClosed
	}

	// If db.maxOpen > 0 and the number of open connections is over the limit
	// and there are no free connection, make a request and wait.
	if db.maxOpen > 0 && db.numOpen >= db.maxOpen && db.freeConn.Len() == 0 {
		// Make the connRequest channel. It's buffered so that the
		// connectionOpener doesn't block while waiting for the req to be read.
		ch := make(chan interface{}, 1)
		req := connRequest(ch)
		db.connRequests.PushBack(req)
		db.maybeOpenNewConnections()
		db.mu.Unlock()
		ret, ok := <-ch
		if !ok {
			return nil, errDBClosed
		}
		switch ret.(type) {
		case *driverConn:
			return ret.(*driverConn), nil
		case error:
			return nil, ret.(error)
		default:
			panic("sql: Unexpected type passed through connRequest.ch")
		}
	}

	if f := db.freeConn.Front(); f != nil {
		conn := f.Value.(*driverConn)
		conn.listElem = nil
		db.freeConn.Remove(f)
		conn.inUse = true
		db.mu.Unlock()
		return conn, nil
	}

	db.mu.Unlock()
	ci, err := db.driver.Open(db.dsn)
	if err != nil {
		return nil, err
	}
	db.mu.Lock()
	db.numOpen++
	dc := &driverConn{
		db: db,
		ci: ci,
	}
	db.addDepLocked(dc, dc)
	dc.inUse = true
	db.mu.Unlock()
	return dc, nil
}

var (
	errConnClosed = errors.New("database/sql: internal sentinel error: conn is closed")
	errConnBusy   = errors.New("database/sql: internal sentinel error: conn is busy")
)

// connIfFree returns (wanted, nil) if wanted is still a valid conn and
// isn't in use.
//
// The error is errConnClosed if the connection if the requested connection
// is invalid because it's been closed.
//
// The error is errConnBusy if the connection is in use.
// TODO：待译
func (db *DB) connIfFree(wanted *driverConn) (*driverConn, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if wanted.dbmuClosed {
		return nil, errConnClosed
	}
	if wanted.inUse {
		return nil, errConnBusy
	}
	if wanted.listElem != nil {
		db.freeConn.Remove(wanted.listElem)
		wanted.listElem = nil
		wanted.inUse = true
		return wanted, nil
	}
	// TODO(bradfitz): shouldn't get here. After Go 1.1, change this to:
	// panic("connIfFree call requested a non-closed, non-busy, non-free conn")
	// Which passes all the tests, but I'm too paranoid to include this
	// late in Go 1.1.
	// Instead, treat it like a busy connection:
	return nil, errConnBusy
}

// putConnHook is a hook for testing.

// putConnHook是一个测试使用的钩子。
var putConnHook func(*DB, *driverConn)

// noteUnusedDriverStatement notes that si is no longer used and should
// be closed whenever possible (when c is next not in use), unless c is
// already closed.
// TODO：待译
func (db *DB) noteUnusedDriverStatement(c *driverConn, si driver.Stmt) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if c.inUse {
		c.onPut = append(c.onPut, func() {
			si.Close()
		})
	} else {
		c.Lock()
		defer c.Unlock()
		if !c.finalClosed {
			si.Close()
		}
	}
}

// debugGetPut determines whether getConn & putConn calls' stack traces
// are returned for more verbose crashes.
// TODO：待译
const debugGetPut = false

// putConn adds a connection to the db's free pool.
// err is optionally the last error that occurred on this connection.

// putConn 将连接加入到数据库的空置池中。
// err 是连接过程中最后遇到的错误。
func (db *DB) putConn(dc *driverConn, err error) {
	db.mu.Lock()
	if !dc.inUse {
		if debugGetPut {
			fmt.Printf("putConn(%v) DUPLICATE was: %s\n\nPREVIOUS was: %s", dc, stack(), db.lastPut[dc])
		}
		panic("sql: connection returned that was never out")
	}
	if debugGetPut {
		db.lastPut[dc] = stack()
	}
	dc.inUse = false

	for _, fn := range dc.onPut {
		fn()
	}
	dc.onPut = nil

	if err == driver.ErrBadConn {
		// Don't reuse bad connections.
		// Since the conn is considered bad and is being discarded, treat it
		// as closed. Don't decrement the open count here, finalClose will
		// take care of that.
		db.maybeOpenNewConnections()
		db.mu.Unlock()
		dc.Close()
		return
	}
	if putConnHook != nil {
		putConnHook(db, dc)
	}
	added := db.putConnDBLocked(dc, nil)
	db.mu.Unlock()

	if !added {
		dc.Close()
	}
}

// Satisfy a connRequest or put the driverConn in the idle pool and return true
// or return false.
// putConnDBLocked will satisfy a connRequest if there is one, or it will
// return the *driverConn to the freeConn list if err == nil and the idle
// connection limit will not be exceeded.
// If err != nil, the value of dc is ignored.
// If err == nil, then dc must not equal nil.
// If a connRequest was fullfilled or the *driverConn was placed in the
// freeConn list, then true is returned, otherwise false is returned.
func (db *DB) putConnDBLocked(dc *driverConn, err error) bool {
	if db.connRequests.Len() > 0 {
		req := db.connRequests.Front().Value.(connRequest)
		db.connRequests.Remove(db.connRequests.Front())
		if err != nil {
			req <- err
		} else {
			dc.inUse = true
			req <- dc
		}
		return true
	} else if err == nil && !db.closed && db.maxIdleConnsLocked() > db.freeConn.Len() {
		dc.listElem = db.freeConn.PushFront(dc)
		return true
	}
	return false
}

// maxBadConnRetries is the number of maximum retries if the driver returns
// driver.ErrBadConn to signal a broken connection.
const maxBadConnRetries = 10

// Prepare creates a prepared statement for later queries or executions.
// Multiple queries or executions may be run concurrently from the
// returned statement.

// Prepare 为以后的查询或执行操作事先创建了语句。
// 多个查询或执行操作可在返回的语句中并发地运行。
func (db *DB) Prepare(query string) (*Stmt, error) {
	var stmt *Stmt
	var err error
	for i := 0; i < maxBadConnRetries; i++ {
		stmt, err = db.prepare(query)
		if err != driver.ErrBadConn {
			break
		}
	}
	return stmt, err
}

func (db *DB) prepare(query string) (*Stmt, error) {
	// TODO: check if db.driver supports an optional
	// driver.Preparer interface and call that instead, if so,
	// otherwise we make a prepared statement that's bound
	// to a connection, and to execute this prepared statement
	// we either need to use this connection (if it's free), else
	// get a new connection + re-prepare + execute on that one.
	dc, err := db.conn()
	if err != nil {
		return nil, err
	}
	dc.Lock()
	si, err := dc.prepareLocked(query)
	dc.Unlock()
	if err != nil {
		db.putConn(dc, err)
		return nil, err
	}
	stmt := &Stmt{
		db:    db,
		query: query,
		css:   []connStmt{{dc, si}},
	}
	db.addDep(stmt, stmt)
	db.putConn(dc, nil)
	return stmt, nil
}

// Exec executes a query without returning any rows.
// The args are for any placeholder parameters in the query.

// Exec 执行query操作，而不返回任何行。
// args 为查询中的任意占位符形参。
func (db *DB) Exec(query string, args ...interface{}) (Result, error) {
	var res Result
	var err error
	for i := 0; i < maxBadConnRetries; i++ {
		res, err = db.exec(query, args)
		if err != driver.ErrBadConn {
			break
		}
	}
	return res, err
}

func (db *DB) exec(query string, args []interface{}) (res Result, err error) {
	dc, err := db.conn()
	if err != nil {
		return nil, err
	}
	defer func() {
		db.putConn(dc, err)
	}()

	if execer, ok := dc.ci.(driver.Execer); ok {
		dargs, err := driverArgs(nil, args)
		if err != nil {
			return nil, err
		}
		dc.Lock()
		resi, err := execer.Exec(query, dargs)
		dc.Unlock()
		if err != driver.ErrSkip {
			if err != nil {
				return nil, err
			}
			return driverResult{dc, resi}, nil
		}
	}

	dc.Lock()
	si, err := dc.ci.Prepare(query)
	dc.Unlock()
	if err != nil {
		return nil, err
	}
	defer withLock(dc, func() { si.Close() })
	return resultFromStatement(driverStmt{dc, si}, args...)
}

// Query executes a query that returns rows, typically a SELECT.
// The args are for any placeholder parameters in the query.

// Query执行了一个有返回行的查询操作，比如SELECT。
// args 形参为该查询中的任何占位符。
func (db *DB) Query(query string, args ...interface{}) (*Rows, error) {
	var rows *Rows
	var err error
	for i := 0; i < maxBadConnRetries; i++ {
		rows, err = db.query(query, args)
		if err != driver.ErrBadConn {
			break
		}
	}
	return rows, err
}

func (db *DB) query(query string, args []interface{}) (*Rows, error) {
	ci, err := db.conn()
	if err != nil {
		return nil, err
	}

	return db.queryConn(ci, ci.releaseConn, query, args)
}

// queryConn executes a query on the given connection.
// The connection gets released by the releaseConn function.
func (db *DB) queryConn(dc *driverConn, releaseConn func(error), query string, args []interface{}) (*Rows, error) {
	if queryer, ok := dc.ci.(driver.Queryer); ok {
		dargs, err := driverArgs(nil, args)
		if err != nil {
			releaseConn(err)
			return nil, err
		}
		dc.Lock()
		rowsi, err := queryer.Query(query, dargs)
		dc.Unlock()
		if err != driver.ErrSkip {
			if err != nil {
				releaseConn(err)
				return nil, err
			}
			// Note: ownership of dc passes to the *Rows, to be freed
			// with releaseConn.
			rows := &Rows{
				dc:          dc,
				releaseConn: releaseConn,
				rowsi:       rowsi,
			}
			return rows, nil
		}
	}

	dc.Lock()
	si, err := dc.ci.Prepare(query)
	dc.Unlock()
	if err != nil {
		releaseConn(err)
		return nil, err
	}

	ds := driverStmt{dc, si}
	rowsi, err := rowsiFromStatement(ds, args...)
	if err != nil {
		dc.Lock()
		si.Close()
		dc.Unlock()
		releaseConn(err)
		return nil, err
	}

	// Note: ownership of ci passes to the *Rows, to be freed
	// with releaseConn.
	rows := &Rows{
		dc:          dc,
		releaseConn: releaseConn,
		rowsi:       rowsi,
		closeStmt:   si,
	}
	return rows, nil
}

// QueryRow executes a query that is expected to return at most one row.
// QueryRow always return a non-nil value. Errors are deferred until
// Row's Scan method is called.

// QueryRow执行一个至多只返回一行记录的查询操作。
// QueryRow总是返回一个非空值。Error只会在调用行的Scan方法的时候才返回。
func (db *DB) QueryRow(query string, args ...interface{}) *Row {
	rows, err := db.Query(query, args...)
	return &Row{rows: rows, err: err}
}

// Begin starts a transaction. The isolation level is dependent on
// the driver.

// Begin开始一个事务。事务的隔离级别是由驱动决定的。
func (db *DB) Begin() (*Tx, error) {
	var tx *Tx
	var err error
	for i := 0; i < maxBadConnRetries; i++ {
		tx, err = db.begin()
		if err != driver.ErrBadConn {
			break
		}
	}
	return tx, err
}

func (db *DB) begin() (tx *Tx, err error) {
	dc, err := db.conn()
	if err != nil {
		return nil, err
	}
	dc.Lock()
	txi, err := dc.ci.Begin()
	dc.Unlock()
	if err != nil {
		db.putConn(dc, err)
		return nil, err
	}
	return &Tx{
		db:  db,
		dc:  dc,
		txi: txi,
	}, nil
}

// Driver returns the database's underlying driver.

// Driver返回了数据库的底层驱动。
func (db *DB) Driver() driver.Driver {
	return db.driver
}

// Tx is an in-progress database transaction.
//
// A transaction must end with a call to Commit or Rollback.
//
// After a call to Commit or Rollback, all operations on the
// transaction fail with ErrTxDone.

// Tx代表运行中的数据库事务。
//
// 必须调用Commit或者Rollback来结束事务。
//
// 在调用 Commit 或者 Rollback 之后，所有对事务的后续操作就会返回 ErrTxDone。
type Tx struct {
	db *DB

	// dc is owned exclusively until Commit or Rollback, at which point
	// it's returned with putConn.

	// 在调用 Commit 或 Rollback 之前，dc 会一直有值，在释放 dc 的时候，它会被
	// putConn 调用返回。
	ci  driver.Conn
	dc  *driverConn
	txi driver.Tx

	// done transitions from false to true exactly once, on Commit
	// or Rollback. once done, all operations fail with
	// ErrTxDone.

	// 一旦Commit或者Rollback，done这个事务标示就会从false值置为true。
	// 一旦这个标志位设置为true，所有事务的操作都会失败并返回ErrTxDone。
	done bool
}

var ErrTxDone = errors.New("sql: Transaction has already been committed or rolled back")

func (tx *Tx) close() {
	if tx.done {
		panic("double close") // internal error
	}
	tx.done = true
	tx.db.putConn(tx.dc, nil)
	tx.dc = nil
	tx.txi = nil
}

func (tx *Tx) grabConn() (*driverConn, error) {
	if tx.done {
		return nil, ErrTxDone
	}
	return tx.dc, nil
}

// Commit commits the transaction.

// Commit提交事务。
func (tx *Tx) Commit() error {
	if tx.done {
		return ErrTxDone
	}
	defer tx.close()
	tx.dc.Lock()
	defer tx.dc.Unlock()
	return tx.txi.Commit()
}

// Rollback aborts the transaction.

// Rollback回滚事务。
func (tx *Tx) Rollback() error {
	if tx.done {
		return ErrTxDone
	}
	defer tx.close()
	tx.dc.Lock()
	defer tx.dc.Unlock()
	return tx.txi.Rollback()
}

// Prepare creates a prepared statement for use within a transaction.
//
// The returned statement operates within the transaction and can no longer
// be used once the transaction has been committed or rolled back.
//
// To use an existing prepared statement on this transaction, see Tx.Stmt.

// Prepare在一个事务中定义了一个操作的声明。
//
// 这里定义的声明操作一旦事务被调用了commited或者rollback之后就不能使用了。
//
// 关于如何使用定义好的操作声明，请参考Tx.Stmt。
func (tx *Tx) Prepare(query string) (*Stmt, error) {
	// TODO(bradfitz): We could be more efficient here and either
	// provide a method to take an existing Stmt (created on
	// perhaps a different Conn), and re-create it on this Conn if
	// necessary. Or, better: keep a map in DB of query string to
	// Stmts, and have Stmt.Execute do the right thing and
	// re-prepare if the Conn in use doesn't have that prepared
	// statement.  But we'll want to avoid caching the statement
	// in the case where we only call conn.Prepare implicitly
	// (such as in db.Exec or tx.Exec), but the caller package
	// can't be holding a reference to the returned statement.
	// Perhaps just looking at the reference count (by noting
	// Stmt.Close) would be enough. We might also want a finalizer
	// on Stmt to drop the reference count.
	dc, err := tx.grabConn()
	if err != nil {
		return nil, err
	}

	dc.Lock()
	si, err := dc.ci.Prepare(query)
	dc.Unlock()
	if err != nil {
		return nil, err
	}

	stmt := &Stmt{
		db: tx.db,
		tx: tx,
		txsi: &driverStmt{
			Locker: dc,
			si:     si,
		},
		query: query,
	}
	return stmt, nil
}

// Stmt returns a transaction-specific prepared statement from
// an existing statement.
//
// Example:
//  updateMoney, err := db.Prepare("UPDATE balance SET money=money+? WHERE id=?")
//  ...
//  tx, err := db.Begin()
//  ...
//  res, err := tx.Stmt(updateMoney).Exec(123.45, 98293203)

// Stmt从一个已有的声明中返回指定事务的声明。
//
// 例子:
//  updateMoney, err := db.Prepare("UPDATE balance SET money=money+? WHERE id=?")
//  ...
//  tx, err := db.Begin()
//  ...
//  res, err := tx.Stmt(updateMoney).Exec(123.45, 98293203)
func (tx *Tx) Stmt(stmt *Stmt) *Stmt {
	// TODO(bradfitz): optimize this. Currently this re-prepares
	// each time.  This is fine for now to illustrate the API but
	// we should really cache already-prepared statements
	// per-Conn. See also the big comment in Tx.Prepare.

	if tx.db != stmt.db {
		return &Stmt{stickyErr: errors.New("sql: Tx.Stmt: statement from different database used")}
	}
	dc, err := tx.grabConn()
	if err != nil {
		return &Stmt{stickyErr: err}
	}
	dc.Lock()
	si, err := dc.ci.Prepare(stmt.query)
	dc.Unlock()
	return &Stmt{
		db: tx.db,
		tx: tx,
		txsi: &driverStmt{
			Locker: dc,
			si:     si,
		},
		query:     stmt.query,
		stickyErr: err,
	}
}

// Exec executes a query that doesn't return rows.
// For example: an INSERT and UPDATE.

// Exec执行不返回任何行的操作。
// 例如：INSERT和UPDATE操作。
func (tx *Tx) Exec(query string, args ...interface{}) (Result, error) {
	dc, err := tx.grabConn()
	if err != nil {
		return nil, err
	}

	if execer, ok := dc.ci.(driver.Execer); ok {
		dargs, err := driverArgs(nil, args)
		if err != nil {
			return nil, err
		}
		dc.Lock()
		resi, err := execer.Exec(query, dargs)
		dc.Unlock()
		if err == nil {
			return driverResult{dc, resi}, nil
		}
		if err != driver.ErrSkip {
			return nil, err
		}
	}

	dc.Lock()
	si, err := dc.ci.Prepare(query)
	dc.Unlock()
	if err != nil {
		return nil, err
	}
	defer withLock(dc, func() { si.Close() })

	return resultFromStatement(driverStmt{dc, si}, args...)
}

// Query executes a query that returns rows, typically a SELECT.

// Query执行哪些返回行的查询操作，比如SELECT。
func (tx *Tx) Query(query string, args ...interface{}) (*Rows, error) {
	dc, err := tx.grabConn()
	if err != nil {
		return nil, err
	}
	releaseConn := func(error) {}
	return tx.db.queryConn(dc, releaseConn, query, args)
}

// QueryRow executes a query that is expected to return at most one row.
// QueryRow always return a non-nil value. Errors are deferred until
// Row's Scan method is called.

// QueryRow执行的查询至多返回一行数据。
// QueryRow总是返回非空值。只有当执行行的Scan方法的时候，才会返回Error。
func (tx *Tx) QueryRow(query string, args ...interface{}) *Row {
	rows, err := tx.Query(query, args...)
	return &Row{rows: rows, err: err}
}

// connStmt is a prepared statement on a particular connection.

// connStmt代表在某个连接上定义好的声明。
type connStmt struct {
	dc *driverConn
	si driver.Stmt
}

// Stmt is a prepared statement. Stmt is safe for concurrent use by multiple goroutines.

// Stmt是定义好的声明。多个goroutine并发使用Stmt是安全的。
type Stmt struct {
	// Immutable:

	// 不变的数据：
	db        *DB    // where we came from	// 数据从哪里来
	query     string // that created the Stmt	// 什么样的查询建立了这个Stmt
	stickyErr error  // if non-nil, this error is returned for all operations  // 如果是非空的话，所有操作都会返回这个错误。

	closemu sync.RWMutex // held exclusively during close, for read otherwise.

	// If in a transaction, else both nil:

	// 只有在事务中，者两个值才都非空，其他情况下都是空的：
	tx   *Tx
	txsi *driverStmt

	mu     sync.Mutex // protects the rest of the fields // 保护其他字段
	closed bool

	// css is a list of underlying driver statement interfaces
	// that are valid on particular connections.  This is only
	// used if tx == nil and one is found that has idle
	// connections.  If tx != nil, txsi is always used.

	// css是一个底层驱动的声明接口的数组，它只对特定的连接有效。只有当tx == nil的时候才使用，
	// 它是从在空闲连接池中获取的。如果tx != nil，就会使用txsi。
	css []connStmt
}

// Exec executes a prepared statement with the given arguments and
// returns a Result summarizing the effect of the statement.

// Exec根据给出的参数执行定义好的声明，并返回Result来显示执行的结果。
func (s *Stmt) Exec(args ...interface{}) (Result, error) {
	s.closemu.RLock()
	defer s.closemu.RUnlock()

	var res Result
	for i := 0; i < maxBadConnRetries; i++ {
		dc, releaseConn, si, err := s.connStmt()
		if err != nil {
			if err == driver.ErrBadConn {
				continue
			}
			return nil, err
		}

		res, err = resultFromStatement(driverStmt{dc, si}, args...)
		releaseConn(err)
		if err != driver.ErrBadConn {
			return res, err
		}
	}
	return nil, driver.ErrBadConn
}

func resultFromStatement(ds driverStmt, args ...interface{}) (Result, error) {
	ds.Lock()
	want := ds.si.NumInput()
	ds.Unlock()

	// -1 means the driver doesn't know how to count the number of
	// placeholders, so we won't sanity check input here and instead let the
	// driver deal with errors.

	// -1意味着驱动不知道如何计算占位符的数量，所以在这里，我们并不检查输入，而是让驱动自己来处理错误。
	if want != -1 && len(args) != want {
		return nil, fmt.Errorf("sql: expected %d arguments, got %d", want, len(args))
	}

	dargs, err := driverArgs(&ds, args)
	if err != nil {
		return nil, err
	}

	ds.Lock()
	resi, err := ds.si.Exec(dargs)
	ds.Unlock()
	if err != nil {
		return nil, err
	}
	return driverResult{ds.Locker, resi}, nil
}

// connStmt returns a free driver connection on which to execute the
// statement, a function to call to release the connection, and a
// statement bound to that connection.

// connStmt返回空闲的驱动连接，这个连接是用来执行这个声明的，并且同时定义一个函数来释放连接，
// 定义一个声明绑定连接。
func (s *Stmt) connStmt() (ci *driverConn, releaseConn func(error), si driver.Stmt, err error) {
	if err = s.stickyErr; err != nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		err = errors.New("sql: statement is closed")
		return
	}

	// In a transaction, we always use the connection that the
	// transaction was created on.

	// 在事务中，我们总是使用事务创建的连接。
	if s.tx != nil {
		s.mu.Unlock()
		ci, err = s.tx.grabConn() // blocks, waiting for the connection. // 阻塞，等待连接。
		if err != nil {
			return
		}
		releaseConn = func(error) {}
		return ci, releaseConn, s.txsi.si, nil
	}

	var cs connStmt
	match := false
	for i := 0; i < len(s.css); i++ {
		v := s.css[i]
		_, err := s.db.connIfFree(v.dc)
		if err == nil {
			match = true
			cs = v
			break
		}
		if err == errConnClosed {
			// Lazily remove dead conn from our freelist.
			s.css[i] = s.css[len(s.css)-1]
			s.css = s.css[:len(s.css)-1]
			i--
		}

	}
	s.mu.Unlock()

	// Make a new conn if all are busy.
	// TODO(bradfitz): or wait for one? make configurable later?
	if !match {
		dc, err := s.db.conn()
		if err != nil {
			return nil, nil, nil, err
		}
		dc.Lock()
		si, err := dc.prepareLocked(s.query)
		dc.Unlock()
		if err != nil {
			s.db.putConn(dc, err)
			return nil, nil, nil, err
		}
		s.mu.Lock()
		cs = connStmt{dc, si}
		s.css = append(s.css, cs)
		s.mu.Unlock()
	}

	conn := cs.dc
	return conn, conn.releaseConn, cs.si, nil
}

// Query executes a prepared query statement with the given arguments
// and returns the query results as a *Rows.

// Query根据传递的参数执行一个声明的查询操作，然后以*Rows的结果返回查询结果。
func (s *Stmt) Query(args ...interface{}) (*Rows, error) {
	s.closemu.RLock()
	defer s.closemu.RUnlock()

	var rowsi driver.Rows
	for i := 0; i < maxBadConnRetries; i++ {
		dc, releaseConn, si, err := s.connStmt()
		if err != nil {
			if err == driver.ErrBadConn {
				continue
			}
			return nil, err
		}

		rowsi, err = rowsiFromStatement(driverStmt{dc, si}, args...)
		if err == nil {
			// Note: ownership of ci passes to the *Rows, to be freed
			// with releaseConn.
			rows := &Rows{
				dc:    dc,
				rowsi: rowsi,
				// releaseConn set below
			}
			s.db.addDep(s, rows)
			rows.releaseConn = func(err error) {
				releaseConn(err)
				s.db.removeDep(s, rows)
			}
			return rows, nil
		}

		releaseConn(err)
		if err != driver.ErrBadConn {
			return nil, err
		}
	}
	return nil, driver.ErrBadConn
}

func rowsiFromStatement(ds driverStmt, args ...interface{}) (driver.Rows, error) {
	ds.Lock()
	want := ds.si.NumInput()
	ds.Unlock()

	// -1 means the driver doesn't know how to count the number of
	// placeholders, so we won't sanity check input here and instead let the
	// driver deal with errors.
	if want != -1 && len(args) != want {
		return nil, fmt.Errorf("sql: statement expects %d inputs; got %d", want, len(args))
	}

	dargs, err := driverArgs(&ds, args)
	if err != nil {
		return nil, err
	}

	ds.Lock()
	rowsi, err := ds.si.Query(dargs)
	ds.Unlock()
	if err != nil {
		return nil, err
	}
	return rowsi, nil
}

// QueryRow executes a prepared query statement with the given arguments.
// If an error occurs during the execution of the statement, that error will
// be returned by a call to Scan on the returned *Row, which is always non-nil.
// If the query selects no rows, the *Row's Scan will return ErrNoRows.
// Otherwise, the *Row's Scan scans the first selected row and discards
// the rest.
//
// Example usage:
//
//  var name string
//  err := nameByUseridStmt.QueryRow(id).Scan(&name)

// QueryRow根据传递的参数执行一个声明的查询操作。如果在执行声明过程中发生了错误，
// 这个error就会在Scan返回的*Row的时候返回，而这个*Row永远不会是nil。
// 如果查询没有任何行数据，*Row的Scan操作就会返回ErrNoRows。
// 否则，*Rows的Scan操作就会返回第一行数据，并且忽略其他行。
//
// Example usage:
//
//  var name string
//  err := nameByUseridStmt.QueryRow(id).Scan(&name)
func (s *Stmt) QueryRow(args ...interface{}) *Row {
	rows, err := s.Query(args...)
	if err != nil {
		return &Row{err: err}
	}
	return &Row{rows: rows}
}

// Close closes the statement.

// 关闭声明。
func (s *Stmt) Close() error {
	s.closemu.Lock()
	defer s.closemu.Unlock()

	if s.stickyErr != nil {
		return s.stickyErr
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true

	if s.tx != nil {
		s.txsi.Close()
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	return s.db.removeDep(s, s)
}

func (s *Stmt) finalClose() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.css != nil {
		for _, v := range s.css {
			s.db.noteUnusedDriverStatement(v.dc, v.si)
			v.dc.removeOpenStmt(v.si)
		}
		s.css = nil
	}
	return nil
}

// Rows is the result of a query. Its cursor starts before the first row
// of the result set. Use Next to advance through the rows:
//
//     rows, err := db.Query("SELECT ...")
//     ...
//     defer rows.Close()
//     for rows.Next() {
//         var id int
//         var name string
//         err = rows.Scan(&id, &name)
//         ...
//     }
//     err = rows.Err() // get any error encountered during iteration
//     ...

// Rows代表查询的结果。它的指针最初指向结果集的第一行数据，需要使用Next来进一步操作。
//
//     rows, err := db.Query("SELECT ...")
//     ...
//     for rows.Next() {
//         var id int
//         var name string
//         err = rows.Scan(&id, &name)
//         ...
//     }
//     err = rows.Err() // get any error encountered during iteration
//     ...
type Rows struct {
	dc          *driverConn // owned; must call releaseConn when closed to release // 已经存在的连接；当释放连接的时候必须调用 releaseConn
	releaseConn func(error)
	rowsi       driver.Rows

	closed    bool
	lastcols  []driver.Value
	lasterr   error       // non-nil only if closed is true // 仅当 closed 为 true 时非 nil
	closeStmt driver.Stmt // if non-nil, statement to Close on close// 若非 nil，该语句会在 Close 调用时关闭
}

// Next prepares the next result row for reading with the Scan method.  It
// returns true on success, or false if there is no next result row or an error
// happened while preparing it.  Err should be consulted to distinguish between
// the two cases.
//
// Every call to Scan, even the first one, must be preceded by a call to Next.

// Next获取下一行的数据以便给Scan调用。
// 在成功的时候返回true，在没有下一行数据，或在准备过程中发生错误时返回false。
// 应通过 Err 来区分这两种情况。
//
// 每次调用来Scan获取数据，甚至是第一行数据，都需要调用Next来处理。
func (rs *Rows) Next() bool {
	if rs.closed {
		return false
	}
	if rs.lastcols == nil {
		rs.lastcols = make([]driver.Value, len(rs.rowsi.Columns()))
	}
	rs.lasterr = rs.rowsi.Next(rs.lastcols)
	if rs.lasterr != nil {
		rs.Close()
		return false
	}
	return true
}

// Err returns the error, if any, that was encountered during iteration.
// Err may be called after an explicit or implicit Close.

// Err 返回错误。如果有错误的话，就会在循环过程中捕获到。
// Err 可能会在一个显式或隐式的 Close 后调用。
func (rs *Rows) Err() error {
	if rs.lasterr == io.EOF {
		return nil
	}
	return rs.lasterr
}

// Columns returns the column names.
// Columns returns an error if the rows are closed, or if the rows
// are from QueryRow and there was a deferred error.

// Columns返回列名字。
// 当rows设置了closed，Columns方法会返回error。
func (rs *Rows) Columns() ([]string, error) {
	if rs.closed {
		return nil, errors.New("sql: Rows are closed")
	}
	if rs.rowsi == nil {
		return nil, errors.New("sql: no Rows available")
	}
	return rs.rowsi.Columns(), nil
}

// Scan copies the columns in the current row into the values pointed
// at by dest.
//
// If an argument has type *[]byte, Scan saves in that argument a copy
// of the corresponding data. The copy is owned by the caller and can
// be modified and held indefinitely. The copy can be avoided by using
// an argument of type *RawBytes instead; see the documentation for
// RawBytes for restrictions on its use.
//
// If an argument has type *interface{}, Scan copies the value
// provided by the underlying driver without conversion. If the value
// is of type []byte, a copy is made and the caller owns the result.

// Scan将当前行的列输出到dest指向的目标值中。
//
// 如果有个参数是*[]byte的类型，Scan在这个参数里面存放的是相关数据的拷贝。
// 这个拷贝是调用函数的人所拥有的，并且可以随时被修改和存取。这个拷贝能避免使用*RawBytes；
// 关于这个类型的使用限制请参考文档。
//
// 如果有个参数是*interface{}类型，Scan会将底层驱动提供的这个值不做任何转换直接拷贝返回。
// 如果值是[]byte类型，Scan就会返回一份拷贝，并且调用者获得返回结果。
func (rs *Rows) Scan(dest ...interface{}) error {
	if rs.closed {
		return errors.New("sql: Rows are closed")
	}
	if rs.lastcols == nil {
		return errors.New("sql: Scan called without calling Next")
	}
	if len(dest) != len(rs.lastcols) {
		return fmt.Errorf("sql: expected %d destination arguments in Scan, not %d", len(rs.lastcols), len(dest))
	}
	for i, sv := range rs.lastcols {
		err := convertAssign(dest[i], sv)
		if err != nil {
			return fmt.Errorf("sql: Scan error on column index %d: %v", i, err)
		}
	}
	return nil
}

var rowsCloseHook func(*Rows, *error)

// Close closes the Rows, preventing further enumeration. If Next returns
// false, the Rows are closed automatically and it will suffice to check the
// result of Err. Close is idempotent and does not affect the result of Err.

// Close 关闭 Rows，阻止了进一步枚举。若 Next 返回 false，则 Rows 会自动关闭并能够检查
// Err 的结果。Close 是幂等的，并不会影响 Err 的结果。
func (rs *Rows) Close() error {
	if rs.closed {
		return nil
	}
	rs.closed = true
	err := rs.rowsi.Close()
	if fn := rowsCloseHook; fn != nil {
		fn(rs, &err)
	}
	if rs.closeStmt != nil {
		rs.closeStmt.Close()
	}
	rs.releaseConn(err)
	return err
}

// Row is the result of calling QueryRow to select a single row.

// Row是调用QueryRow的结果，代表了查询操作的一行数据。
type Row struct {
	// One of these two will be non-nil:

	// 这两个中的一个必须是非空：
	err  error // deferred error for easy chaining  // 将error保存从而延迟返回，这样能保证Row链表的简易实现
	rows *Rows
}

// Scan copies the columns from the matched row into the values
// pointed at by dest.  If more than one row matches the query,
// Scan uses the first row and discards the rest.  If no row matches
// the query, Scan returns ErrNoRows.

// Scan将符合的行的对应列拷贝到dest指的对应值中。
// 如果多于一个的行满足查询条件，Scan使用第一行，而忽略其他行。
// 如果没有行满足查询条件，Scan返回ErrNoRows。
func (r *Row) Scan(dest ...interface{}) error {
	if r.err != nil {
		return r.err
	}

	// TODO(bradfitz): for now we need to defensively clone all
	// []byte that the driver returned (not permitting
	// *RawBytes in Rows.Scan), since we're about to close
	// the Rows in our defer, when we return from this function.
	// the contract with the driver.Next(...) interface is that it
	// can return slices into read-only temporary memory that's
	// only valid until the next Scan/Close.  But the TODO is that
	// for a lot of drivers, this copy will be unnecessary.  We
	// should provide an optional interface for drivers to
	// implement to say, "don't worry, the []bytes that I return
	// from Next will not be modified again." (for instance, if
	// they were obtained from the network anyway) But for now we
	// don't care.
	defer r.rows.Close()
	for _, dp := range dest {
		if _, ok := dp.(*RawBytes); ok {
			return errors.New("sql: RawBytes isn't allowed on Row.Scan")
		}
	}

	if !r.rows.Next() {
		if err := r.rows.Err(); err != nil {
			return err
		}
		return ErrNoRows
	}
	err := r.rows.Scan(dest...)
	if err != nil {
		return err
	}
	// Make sure the query can be processed to completion with no errors.
	if err := r.rows.Close(); err != nil {
		return err
	}

	return nil
}

// A Result summarizes an executed SQL command.

// 一个Result结构代表了一个执行过的SQL命令。
type Result interface {
	// LastInsertId returns the integer generated by the database
	// in response to a command. Typically this will be from an
	// "auto increment" column when inserting a new row. Not all
	// databases support this feature, and the syntax of such
	// statements varies.
	LastInsertId() (int64, error)

	// RowsAffected returns the number of rows affected by an
	// update, insert, or delete. Not every database or database
	// driver may support this.
	RowsAffected() (int64, error)
}

type driverResult struct {
	sync.Locker // the *driverConn
	resi        driver.Result
}

func (dr driverResult) LastInsertId() (int64, error) {
	dr.Lock()
	defer dr.Unlock()
	return dr.resi.LastInsertId()
}

func (dr driverResult) RowsAffected() (int64, error) {
	dr.Lock()
	defer dr.Unlock()
	return dr.resi.RowsAffected()
}

func stack() string {
	var buf [2 << 10]byte
	return string(buf[:runtime.Stack(buf[:], false)])
}

// withLock runs while holding lk.
func withLock(lk sync.Locker, fn func()) {
	lk.Lock()
	fn()
	lk.Unlock()
}
