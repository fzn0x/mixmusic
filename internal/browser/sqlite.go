package browser

// This file reads the tables of an SQLite database, as Firefox keeps its
// cookies in, from the bytes of its file and of its write-ahead log, as
// https://www.sqlite.org/fileformat2.html describes them. It reads, and
// only what the cookies need: the rows of tables that have row IDs, in
// UTF-8. What it reads may be damaged, or half written as Firefox writes:
// it then says so, and never reads past what it was given.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"math"
	"strings"
)

const sqliteMagic = "SQLite format 3\x00"

// The first words of a write-ahead log, which tell how its checksums read
// its words: little-endian, or big-endian.
const (
	walLittleEndian = 0x377f0682
	walBigEndian    = 0x377f0683
)

var (
	errNotSQLite = errors.New("not an SQLite database")
	errDamaged   = errors.New("the SQLite database is damaged, or half written")
)

// maxPayload is the size of the largest row read: far more than a cookie.
const maxPayload = 1 << 24

// sqliteDB is a database as SQLite reads it at a moment: the pages of its
// file, but those its log holds since, as of the log's last commit.
type sqliteDB struct {
	pageSize, usable int
	file             []byte
	log              map[uint32][]byte
}

// openSQLite reads a database from its file and its write-ahead log, nil
// when it has none. The file of a new database may hold nothing yet but
// what the log holds.
func openSQLite(file, wal []byte) (*sqliteDB, error) {
	db := &sqliteDB{file: file}
	switch {
	case len(file) >= 100 && string(file[:16]) == sqliteMagic:
		db.pageSize = pageSizeOf(binary.BigEndian.Uint16(file[16:]))
	case len(wal) >= 32:
		db.pageSize = int(binary.BigEndian.Uint32(wal[8:]))
	}
	if db.pageSize < 512 || db.pageSize > 65536 || db.pageSize&(db.pageSize-1) != 0 {
		return nil, errNotSQLite
	}
	db.log = readLog(wal, db.pageSize)
	first, err := db.page(1)
	if err != nil {
		return nil, err
	}
	if string(first[:16]) != sqliteMagic || pageSizeOf(binary.BigEndian.Uint16(first[16:])) != db.pageSize {
		return nil, errNotSQLite
	}
	db.usable = db.pageSize - int(first[20])
	if db.usable < 480 {
		return nil, errDamaged
	}
	// 1 is UTF-8; 0, of a database with no text yet, too.
	if enc := binary.BigEndian.Uint32(first[56:]); enc > 1 {
		return nil, errors.New("the SQLite database's text isn't in UTF-8")
	}
	return db, nil
}

// pageSizeOf reads the page size of a database's header, where 1 stands
// for 65536.
func pageSizeOf(v uint16) int {
	if v == 1 {
		return 65536
	}
	return int(v)
}

// readLog returns the pages of a write-ahead log as of its last commit: of
// its frames, those whose checksums and salts follow on from its header,
// up to the last that ends a transaction.
func readLog(wal []byte, pageSize int) map[uint32][]byte {
	if len(wal) < 32 {
		return nil
	}
	var order binary.ByteOrder
	switch binary.BigEndian.Uint32(wal) {
	case walLittleEndian:
		order = binary.LittleEndian
	case walBigEndian:
		order = binary.BigEndian
	default:
		return nil
	}
	be := binary.BigEndian
	if int(be.Uint32(wal[8:])) != pageSize {
		return nil
	}
	s0, s1 := walSum(order, wal[:24], 0, 0)
	if s0 != be.Uint32(wal[24:]) || s1 != be.Uint32(wal[28:]) {
		return nil
	}
	committed, pending := map[uint32][]byte{}, map[uint32][]byte{}
	for off := 32; off+24+pageSize <= len(wal); off += 24 + pageSize {
		frame, page := wal[off:off+24], wal[off+24:off+24+pageSize]
		if !bytes.Equal(frame[8:16], wal[16:24]) {
			break
		}
		s0, s1 = walSum(order, frame[:8], s0, s1)
		s0, s1 = walSum(order, page, s0, s1)
		if s0 != be.Uint32(frame[16:]) || s1 != be.Uint32(frame[20:]) {
			break
		}
		pending[be.Uint32(frame)] = page
		if be.Uint32(frame[4:]) != 0 { // the frame that commits
			maps.Copy(committed, pending)
			clear(pending)
		}
	}
	return committed
}

// walSum goes on with the checksum s0, s1 of a write-ahead log over b.
func walSum(order binary.ByteOrder, b []byte, s0, s1 uint32) (uint32, uint32) {
	for i := 0; i+8 <= len(b); i += 8 {
		s0 += order.Uint32(b[i:]) + s1
		s1 += order.Uint32(b[i+4:]) + s0
	}
	return s0, s1
}

// page returns the page numbered n, from 1.
func (db *sqliteDB) page(n uint32) ([]byte, error) {
	if p, ok := db.log[n]; ok {
		return p, nil
	}
	off := int64(n-1) * int64(db.pageSize)
	if n == 0 || off+int64(db.pageSize) > int64(len(db.file)) {
		return nil, errDamaged
	}
	return db.file[off : off+int64(db.pageSize)], nil
}

// sqliteRow is a row of a table, whose values its columns name.
type sqliteRow struct {
	columns map[string]int
	values  []any
}

func (r sqliteRow) value(column string) any {
	if i, ok := r.columns[column]; ok && i < len(r.values) {
		return r.values[i]
	}
	return nil // a column added since the row was written
}

// text returns a column's text, or "".
func (r sqliteRow) text(column string) string {
	switch v := r.value(column).(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	return ""
}

// integer returns a column's number, or 0.
func (r sqliteRow) integer(column string) int64 {
	switch v := r.value(column).(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	}
	return 0
}

// table returns the rows of the table name.
func (db *sqliteDB) table(name string) ([]sqliteRow, error) {
	// The schema is the table of the first page: type, name, the table's
	// name, its root page and the SQL that made it.
	var root int64
	var sql string
	err := db.walk(1, func(_ int64, payload []byte) error {
		v, err := record(payload)
		if err == nil && len(v) >= 5 && v[0] == "table" && v[1] == name {
			root, _ = v[3].(int64)
			sql, _ = v[4].(string)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if root <= 0 || root > math.MaxUint32 {
		return nil, fmt.Errorf("the SQLite database has no table %s", name)
	}
	names, rowid := columnsOf(sql)
	columns := make(map[string]int, len(names))
	for i, n := range names {
		columns[n] = i
	}
	var rows []sqliteRow
	err = db.walk(uint32(root), func(id int64, payload []byte) error {
		v, err := record(payload)
		if err != nil {
			return err
		}
		// A column that is the row ID is kept as NULL.
		if rowid >= 0 && rowid < len(v) && v[rowid] == nil {
			v[rowid] = id
		}
		rows = append(rows, sqliteRow{columns, v})
		return nil
	})
	return rows, err
}

// walk calls row with the ID and the payload of each row of the table
// whose b-tree begins at the page root.
func (db *sqliteDB) walk(root uint32, row func(id int64, payload []byte) error) error {
	be := binary.BigEndian
	seen := map[uint32]bool{}
	var visit func(n uint32) error
	visit = func(n uint32) error {
		if seen[n] || len(seen) > 1<<20 {
			return errDamaged
		}
		seen[n] = true
		p, err := db.page(n)
		if err != nil {
			return err
		}
		h := 0
		if n == 1 {
			h = 100 // after the database's header
		}
		cells := int(be.Uint16(p[h+3:]))
		switch p[h] {
		case 0x0d: // a leaf, of rows
			ptrs := h + 8
			if ptrs+2*cells > db.usable {
				return errDamaged
			}
			for i := range cells {
				if err := db.leafCell(p, int(be.Uint16(p[ptrs+2*i:])), row); err != nil {
					return err
				}
			}
			return nil
		case 0x05: // an interior page, of the pages below it
			ptrs := h + 12
			if ptrs+2*cells > db.usable {
				return errDamaged
			}
			for i := range cells {
				off := int(be.Uint16(p[ptrs+2*i:]))
				if off+4 > db.usable {
					return errDamaged
				}
				if err := visit(be.Uint32(p[off:])); err != nil {
					return err
				}
			}
			return visit(be.Uint32(p[h+8:]))
		}
		return errDamaged
	}
	return visit(root)
}

// leafCell reads the row of a leaf's cell at off of its page p, with what
// of its payload overflows into other pages.
func (db *sqliteDB) leafCell(p []byte, off int, row func(int64, []byte) error) error {
	if off >= db.usable {
		return errDamaged
	}
	size, n := uvarint(p[off:db.usable])
	if n == 0 || size > maxPayload {
		return errDamaged
	}
	off += n
	id, n := uvarint(p[off:db.usable])
	if n == 0 {
		return errDamaged
	}
	off += n
	local := db.localSize(int(size))
	if off+local > db.usable {
		return errDamaged
	}
	payload := append(make([]byte, 0, size), p[off:off+local]...)
	if local < int(size) {
		if off+local+4 > db.usable {
			return errDamaged
		}
		next := binary.BigEndian.Uint32(p[off+local:])
		for len(payload) < int(size) {
			if next == 0 {
				return errDamaged
			}
			over, err := db.page(next)
			if err != nil {
				return err
			}
			next = binary.BigEndian.Uint32(over)
			take := min(int(size)-len(payload), db.usable-4)
			payload = append(payload, over[4:4+take]...)
		}
	}
	return row(int64(id), payload)
}

// localSize returns how much of a payload of size bytes a leaf of a table
// keeps itself.
func (db *sqliteDB) localSize(size int) int {
	u := db.usable
	most := u - 35
	if size <= most {
		return size
	}
	least := (u-12)*32/255 - 23
	k := least + (size-least)%(u-4)
	if k <= most {
		return k
	}
	return least
}

// record reads the values of a row's payload: nil, int64, float64, string
// or []byte.
func record(payload []byte) ([]any, error) {
	size, n := uvarint(payload)
	if n == 0 || size < uint64(n) || size > uint64(len(payload)) {
		return nil, errDamaged
	}
	header, body := payload[n:size], payload[size:]
	var values []any
	for len(header) > 0 {
		t, n := uvarint(header)
		if n == 0 {
			return nil, errDamaged
		}
		header = header[n:]
		var v any
		length := 0
		switch {
		case t == 0:
		case t <= 6:
			length = [...]int{0, 1, 2, 3, 4, 6, 8}[t]
		case t == 7 || t >= 12:
			length = 8
			if t >= 12 {
				length = int((t - 12) / 2)
			}
		case t == 8, t == 9:
			v = int64(t - 8)
		default:
			return nil, errDamaged
		}
		if length > len(body) || length < 0 {
			return nil, errDamaged
		}
		data := body[:length]
		body = body[length:]
		switch {
		case t >= 1 && t <= 6:
			v = signed(data)
		case t == 7:
			v = math.Float64frombits(binary.BigEndian.Uint64(data))
		case t >= 12 && t%2 == 0:
			v = bytes.Clone(data)
		case t >= 13:
			v = string(data)
		}
		values = append(values, v)
	}
	return values, nil
}

// signed reads a big-endian two's complement integer.
func signed(b []byte) int64 {
	var v int64
	if len(b) > 0 && b[0]&0x80 != 0 {
		v = -1
	}
	for _, c := range b {
		v = v<<8 | int64(c)
	}
	return v
}

// uvarint reads SQLite's variable-length integer, of up to 9 bytes, the
// last of which gives all its 8 bits: the value, and how many bytes it
// took, or 0 when b ends first.
func uvarint(b []byte) (uint64, int) {
	var v uint64
	for i := 0; i < 9 && i < len(b); i++ {
		if i == 8 {
			return v<<8 | uint64(b[i]), 9
		}
		v = v<<7 | uint64(b[i]&0x7f)
		if b[i] < 0x80 {
			return v, i + 1
		}
	}
	return 0, 0
}

// columnsOf returns the names of the columns of the table that sql, its
// CREATE TABLE statement, made, and which of them is the row ID, as
// INTEGER PRIMARY KEY makes one, or -1.
func columnsOf(sql string) ([]string, int) {
	open, end := strings.IndexByte(sql, '('), strings.LastIndexByte(sql, ')')
	if open < 0 || end <= open {
		return nil, -1
	}
	var names []string
	rowid := -1
	for _, def := range splitTop(sql[open+1 : end]) {
		f := strings.Fields(def)
		if len(f) == 0 {
			continue
		}
		switch strings.ToUpper(f[0]) {
		case "CONSTRAINT", "PRIMARY", "UNIQUE", "CHECK", "FOREIGN":
			continue // a constraint of the table, not a column
		}
		if strings.HasPrefix(strings.ToUpper(strings.Join(f[1:], " ")), "INTEGER PRIMARY KEY") {
			rowid = len(names)
		}
		names = append(names, strings.Trim(f[0], "\"`[]"))
	}
	return names, rowid
}

// splitTop splits the definitions of a table's columns at the commas that
// aren't in brackets or quotes.
func splitTop(s string) []string {
	var parts []string
	depth, start := 0, 0
	var quote byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"' || c == '`':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ',' && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}
