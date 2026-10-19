// Package xls reads cell values from a legacy Excel (.xls) workbook using only
// the standard library. It covers the subset that published data workbooks use:
// OLE2 compound storage, BIFF8 records, and the SST shared string table. It is
// deliberately small and read-only; formatting, formulas, and other record types
// are ignored.
package xls

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Read parses every worksheet, returning rows of cell text keyed by sheet name.
func Read(data []byte) (map[string][][]string, error) {
	ole, err := newOLE(data)
	if err != nil {
		return nil, err
	}
	book, err := ole.stream("Workbook")
	if err != nil {
		book, err = ole.stream("Book") // BIFF5 fallback
		if err != nil {
			return nil, fmt.Errorf("xls: no workbook stream: %w", err)
		}
	}
	return parseWorkbook(book)
}

// --- OLE2 compound file ---------------------------------------------------------

type ole struct {
	data       []byte
	sectorSize int
	miniSize   int
	miniCutoff int
	fat        []uint32
	miniFat    []uint32
	dir        []dirEntry
	miniStream []byte
}

type dirEntry struct {
	name  string
	typ   byte
	start uint32
	size  uint64
}

const (
	endOfChain = 0xFFFFFFFE
	freeSect   = 0xFFFFFFFF
)

func newOLE(data []byte) (*ole, error) {
	if len(data) < 512 || binary.LittleEndian.Uint64(data) != 0xE11AB1A1E011CFD0 {
		return nil, errors.New("xls: not an OLE2 compound file")
	}
	o := &ole{data: data}
	o.sectorSize = 1 << binary.LittleEndian.Uint16(data[0x1E:])
	o.miniSize = 1 << binary.LittleEndian.Uint16(data[0x20:])
	numFAT := binary.LittleEndian.Uint32(data[0x2C:])
	firstDir := binary.LittleEndian.Uint32(data[0x30:])
	o.miniCutoff = int(binary.LittleEndian.Uint32(data[0x38:]))
	firstMiniFAT := binary.LittleEndian.Uint32(data[0x3C:])
	numMiniFAT := binary.LittleEndian.Uint32(data[0x40:])
	firstDIFAT := binary.LittleEndian.Uint32(data[0x44:])
	numDIFAT := binary.LittleEndian.Uint32(data[0x48:])

	difat := make([]uint32, 0, numFAT)
	for i := 0; i < 109; i++ {
		difat = append(difat, binary.LittleEndian.Uint32(data[0x4C+i*4:]))
	}
	next := firstDIFAT
	for n := uint32(0); n < numDIFAT && next != endOfChain && next != freeSect; n++ {
		sect := o.sector(next)
		if sect == nil {
			break
		}
		per := o.sectorSize/4 - 1
		for i := 0; i < per; i++ {
			difat = append(difat, binary.LittleEndian.Uint32(sect[i*4:]))
		}
		next = binary.LittleEndian.Uint32(sect[o.sectorSize-4:])
	}

	for i := uint32(0); i < numFAT; i++ {
		if int(i) >= len(difat) {
			break
		}
		sect := o.sector(difat[i])
		if sect == nil {
			break
		}
		for j := 0; j+4 <= o.sectorSize; j += 4 {
			o.fat = append(o.fat, binary.LittleEndian.Uint32(sect[j:]))
		}
	}
	if len(o.fat) == 0 {
		return nil, errors.New("xls: empty FAT")
	}

	if err := o.readDirectory(firstDir); err != nil {
		return nil, err
	}
	if err := o.readMini(firstMiniFAT, numMiniFAT); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *ole) sector(n uint32) []byte {
	off := int(n+1) * o.sectorSize
	if n >= endOfChain || off < 0 || off+o.sectorSize > len(o.data) {
		return nil
	}
	return o.data[off : off+o.sectorSize]
}

// readChain walks the FAT (or mini FAT) from start, concatenating sectors.
func (o *ole) readChain(start uint32, mini bool, size int) []byte {
	var out []byte
	seen := 0
	limit := len(o.fat) + 4
	cur := start
	for cur != endOfChain && cur != freeSect && seen < limit {
		var sect []byte
		if mini {
			sect = o.miniSector(cur)
		} else {
			sect = o.sector(cur)
		}
		if sect == nil {
			break
		}
		out = append(out, sect...)
		seen++
		fat := o.fat
		if mini {
			fat = o.miniFat
		}
		if int(cur) >= len(fat) {
			break
		}
		cur = fat[cur]
	}
	if size >= 0 && len(out) > size {
		out = out[:size]
	}
	return out
}

func (o *ole) miniSector(n uint32) []byte {
	off := int(n) * o.miniSize
	if off < 0 || off+o.miniSize > len(o.miniStream) {
		return nil
	}
	return o.miniStream[off : off+o.miniSize]
}

func (o *ole) readDirectory(first uint32) error {
	raw := o.readChain(first, false, -1)
	for off := 0; off+128 <= len(raw); off += 128 {
		e := raw[off : off+128]
		typ := e[0x42]
		if typ == 0 {
			continue
		}
		nameLen := int(binary.LittleEndian.Uint16(e[0x40:]))
		name := decodeUTF16(e[:0x40], nameLen)
		o.dir = append(o.dir, dirEntry{
			name:  name,
			typ:   typ,
			start: binary.LittleEndian.Uint32(e[0x74:]),
			size:  binary.LittleEndian.Uint64(e[0x78:]),
		})
	}
	if len(o.dir) == 0 {
		return errors.New("xls: empty directory")
	}
	// The root entry's stream is the mini stream that backs small streams.
	for _, e := range o.dir {
		if e.typ == 5 { // root storage
			o.miniStream = o.readChain(e.start, false, int(e.size))
			break
		}
	}
	return nil
}

func (o *ole) readMini(first uint32, count uint32) error {
	cur := first
	for n := uint32(0); n < count && cur != endOfChain && cur != freeSect; n++ {
		sect := o.sector(cur)
		if sect == nil {
			break
		}
		for j := 0; j+4 <= o.sectorSize; j += 4 {
			o.miniFat = append(o.miniFat, binary.LittleEndian.Uint32(sect[j:]))
		}
		if int(cur) >= len(o.fat) {
			break
		}
		cur = o.fat[cur]
	}
	return nil
}

func (o *ole) stream(name string) ([]byte, error) {
	for _, e := range o.dir {
		if e.name != name || e.typ != 2 {
			continue
		}
		if e.size < uint64(o.miniCutoff) && len(o.miniStream) > 0 {
			return o.readChain(e.start, true, int(e.size)), nil
		}
		return o.readChain(e.start, false, int(e.size)), nil
	}
	return nil, fmt.Errorf("xls: stream %q not found", name)
}

func decodeUTF16(b []byte, nameLen int) string {
	n := nameLen/2 - 1
	if n < 0 {
		n = 0
	}
	if 2*n > len(b) {
		n = len(b) / 2
	}
	u := make([]uint16, n)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u))
}

// --- BIFF8 records ---------------------------------------------------------------

const (
	recFormula   = 0x0006
	recEOF       = 0x000A
	recBOF       = 0x0809
	recBound     = 0x0085
	recMulRK     = 0x00BD
	recMulBlank  = 0x00BE
	recLabelSST  = 0x00FD
	recSST       = 0x00FC
	recContinue  = 0x003C
	recLabel     = 0x0204
	recString    = 0x0207
	recNumber    = 0x0203
	recRK        = 0x027E
	recBlank     = 0x0201
	recBoolErr   = 0x0205
	substrHeader = 0x0005
	substrSheet  = 0x0010
)

type record struct {
	id   uint16
	data []byte
}

// records splits a BIFF stream into records.
func records(b []byte) []record {
	var out []record
	for len(b) >= 4 {
		id := binary.LittleEndian.Uint16(b)
		n := int(binary.LittleEndian.Uint16(b[2:]))
		if 4+n > len(b) {
			break
		}
		out = append(out, record{id: id, data: b[4 : 4+n]})
		b = b[4+n:]
	}
	return out
}

type sheetRef struct {
	name   string
	offset int
}

type parsed struct {
	sst    []string
	sheets []sheetRef
}

func parseWorkbook(stream []byte) (map[string][][]string, error) {
	p, err := scanGlobals(stream)
	if err != nil {
		return nil, err
	}
	out := make(map[string][][]string, len(p.sheets))
	for i, sh := range p.sheets {
		end := len(stream)
		if i+1 < len(p.sheets) {
			end = p.sheets[i+1].offset
		}
		if sh.offset < 0 || sh.offset >= end {
			continue
		}
		out[sh.name] = parseSheet(stream[sh.offset:end], p.sst)
	}
	return out, nil
}

// scanGlobals walks the workbook globals substream for the SST and the sheet
// directory, recording each sheet's stream offset.
func scanGlobals(stream []byte) (*parsed, error) {
	recs := records(stream)
	p := &parsed{}
	inGlobals := false
	for i, r := range recs {
		switch r.id {
		case recBOF:
			if len(r.data) >= 4 {
				inGlobals = binary.LittleEndian.Uint16(r.data[2:]) == substrHeader
			}
			if !inGlobals {
				// Reached the worksheet substreams; stop scanning globals.
				return p, nil
			}
		case recSST:
			if inGlobals {
				p.sst = parseSST(collectContinue(recs, i))
			}
		case recBound:
			if inGlobals && len(r.data) >= 8 {
				off := int(binary.LittleEndian.Uint32(r.data))
				name := parseShortString(r.data[6:])
				p.sheets = append(p.sheets, sheetRef{name: name, offset: off})
			}
		}
	}
	return p, nil
}

// collectContinue returns the SST payload plus any trailing CONTINUE payloads.
func collectContinue(recs []record, i int) [][]byte {
	blocks := [][]byte{recs[i].data}
	for j := i + 1; j < len(recs) && recs[j].id == recContinue; j++ {
		blocks = append(blocks, recs[j].data)
	}
	return blocks
}

func parseSheet(b []byte, sst []string) [][]string {
	recs := records(b)
	type cell struct {
		row, col int
		text     string
	}
	var cells []cell
	maxRow, maxCol := -1, -1
	var pending *cell // for FORMULA awaiting a STRING record
	for _, r := range recs {
		if r.id == recEOF {
			break
		}
		switch r.id {
		case recLabelSST:
			if len(r.data) < 10 {
				continue
			}
			c := cell{
				row: int(binary.LittleEndian.Uint16(r.data)),
				col: int(binary.LittleEndian.Uint16(r.data[2:])),
			}
			if idx := int(binary.LittleEndian.Uint32(r.data[6:])); idx >= 0 && idx < len(sst) {
				c.text = sst[idx]
			}
			cells = append(cells, c)
			pending = nil
		case recLabel:
			if len(r.data) < 8 {
				continue
			}
			c := cell{
				row:  int(binary.LittleEndian.Uint16(r.data)),
				col:  int(binary.LittleEndian.Uint16(r.data[2:])),
				text: parseUnicodeString(r.data[6:]),
			}
			cells = append(cells, c)
			pending = nil
		case recNumber:
			if len(r.data) < 14 {
				continue
			}
			v := math.Float64frombits(binary.LittleEndian.Uint64(r.data[6:]))
			cells = append(cells, cell{
				row:  int(binary.LittleEndian.Uint16(r.data)),
				col:  int(binary.LittleEndian.Uint16(r.data[2:])),
				text: formatFloat(v),
			})
			pending = nil
		case recRK:
			if len(r.data) < 10 {
				continue
			}
			cells = append(cells, cell{
				row:  int(binary.LittleEndian.Uint16(r.data)),
				col:  int(binary.LittleEndian.Uint16(r.data[2:])),
				text: formatFloat(decodeRK(binary.LittleEndian.Uint32(r.data[6:]))),
			})
			pending = nil
		case recMulRK:
			if len(r.data) < 6 {
				continue
			}
			row := int(binary.LittleEndian.Uint16(r.data))
			col := int(binary.LittleEndian.Uint16(r.data[2:]))
			// Each cell is 6 bytes (xf + rk), then a trailing colLast (2 bytes).
			for off := 4; off+10 <= len(r.data); off += 6 {
				v := decodeRK(binary.LittleEndian.Uint32(r.data[off+2:]))
				cells = append(cells, cell{row: row, col: col, text: formatFloat(v)})
				col++
			}
			pending = nil
		case recFormula:
			if len(r.data) < 14 {
				continue
			}
			c := cell{
				row: int(binary.LittleEndian.Uint16(r.data)),
				col: int(binary.LittleEndian.Uint16(r.data[2:])),
			}
			res := r.data[6:14]
			// 0xFFFF followed by type 0 means the result is a string in a
			// following STRING record.
			if res[6] == 0xFF && res[7] == 0xFF && res[0] == 0 {
				pending = &c
				continue
			}
			c.text = formatFloat(math.Float64frombits(binary.LittleEndian.Uint64(res)))
			cells = append(cells, c)
			pending = nil
		case recString:
			if pending != nil {
				pending.text = parseUnicodeString(r.data)
				cells = append(cells, *pending)
				pending = nil
			}
		}
	}
	for _, c := range cells {
		if c.row > maxRow {
			maxRow = c.row
		}
		if c.col > maxCol {
			maxCol = c.col
		}
	}
	if maxRow < 0 || maxCol < 0 {
		return nil
	}
	grid := make([][]string, maxRow+1)
	for i := range grid {
		grid[i] = make([]string, maxCol+1)
	}
	for _, c := range cells {
		grid[c.row][c.col] = c.text
	}
	return grid
}

// --- BIFF string decoding --------------------------------------------------------

// sstReader walks the SST payload and any CONTINUE blocks. When a string's
// characters straddle a block boundary, the continuation begins with a one-byte
// option field that resets the character encoding for the remainder.
type sstReader struct {
	blocks [][]byte
	bi     int
	off    int
}

func (r *sstReader) advance() bool {
	for r.bi < len(r.blocks) {
		if r.off < len(r.blocks[r.bi]) {
			return true
		}
		r.bi++
		r.off = 0
	}
	return false
}

func (r *sstReader) read(n int) []byte {
	out := make([]byte, 0, n)
	for len(out) < n {
		if !r.advance() {
			break
		}
		take := len(r.blocks[r.bi]) - r.off
		if take > n-len(out) {
			take = n - len(out)
		}
		out = append(out, r.blocks[r.bi][r.off:r.off+take]...)
		r.off += take
	}
	return out
}

func (r *sstReader) u8() int {
	b := r.read(1)
	if len(b) == 0 {
		return 0
	}
	return int(b[0])
}

// nextBlock moves to the following CONTINUE block mid-string, consuming the
// option byte that resets the compression flag. It reports whether it moved.
func (r *sstReader) nextBlock(compressed bool) (bool, bool) {
	if r.off < len(r.blocks[r.bi]) {
		return false, compressed
	}
	r.bi++
	r.off = 0
	if r.bi >= len(r.blocks) {
		return false, compressed
	}
	if len(r.blocks[r.bi]) == 0 {
		return false, compressed
	}
	opt := r.blocks[r.bi][0]
	r.off = 1
	return true, opt&0x01 == 0
}

// chars reads n characters, following block boundaries and their option bytes.
func (r *sstReader) chars(n int, compressed bool) string {
	var b strings.Builder
	for i := 0; i < n; {
		moved, comp := r.nextBlock(compressed)
		_ = moved
		compressed = comp
		if r.bi >= len(r.blocks) {
			break
		}
		avail := len(r.blocks[r.bi]) - r.off
		width := 2
		if compressed {
			width = 1
		}
		can := avail / width
		if can > n-i {
			can = n - i
		}
		if can <= 0 {
			// Not enough for a full character here; the next loop moves on.
			r.off = len(r.blocks[r.bi])
			continue
		}
		chunk := r.blocks[r.bi][r.off : r.off+can*width]
		if compressed {
			for _, c := range chunk {
				b.WriteRune(rune(c))
			}
		} else {
			u := make([]uint16, can)
			for j := 0; j < can; j++ {
				u[j] = binary.LittleEndian.Uint16(chunk[j*2:])
			}
			b.WriteString(string(utf16.Decode(u)))
		}
		r.off += can * width
		i += can
	}
	return b.String()
}

func parseSST(blocks [][]byte) []string {
	if len(blocks) == 0 || len(blocks[0]) < 8 {
		return nil
	}
	unique := int(binary.LittleEndian.Uint32(blocks[0][4:]))
	r := &sstReader{blocks: blocks, off: 8}
	out := make([]string, 0, unique)
	for i := 0; i < unique; i++ {
		if !r.advance() {
			break
		}
		cch := int(binary.LittleEndian.Uint16(r.read(2)))
		grbit := r.u8()
		compressed := grbit&0x01 == 0
		rich := grbit&0x08 != 0
		ext := grbit&0x04 != 0
		s := r.chars(cch, compressed)
		if rich {
			nRuns := int(binary.LittleEndian.Uint16(r.read(2)))
			r.read(nRuns * 4)
		}
		if ext {
			r.read(int(binary.LittleEndian.Uint32(r.read(4))))
		}
		out = append(out, s)
	}
	return out
}

// parseUnicodeString decodes an XLUnicodeString (two-byte length) from the front.
func parseUnicodeString(b []byte) string {
	if len(b) < 3 {
		return ""
	}
	cch := int(binary.LittleEndian.Uint16(b))
	grbit := b[2]
	compressed := grbit&0x01 == 0
	p := b[3:]
	if cch <= 0 {
		return ""
	}
	if compressed {
		if len(p) < cch {
			cch = len(p)
		}
		return string(p[:cch])
	}
	if len(p) < cch*2 {
		cch = len(p) / 2
	}
	u := make([]uint16, cch)
	for i := 0; i < cch; i++ {
		u[i] = binary.LittleEndian.Uint16(p[i*2:])
	}
	return string(utf16.Decode(u))
}

// parseShortString decodes an XLSUnicodeShortString (one-byte length), used for
// sheet names in BOUNDSHEET records.
func parseShortString(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	cch := int(b[0])
	grbit := b[1]
	compressed := grbit&0x01 == 0
	p := b[2:]
	if compressed {
		if len(p) < cch {
			cch = len(p)
		}
		return string(p[:cch])
	}
	if len(p) < cch*2 {
		cch = len(p) / 2
	}
	u := make([]uint16, cch)
	for i := 0; i < cch; i++ {
		u[i] = binary.LittleEndian.Uint16(p[i*2:])
	}
	return string(utf16.Decode(u))
}

func decodeRK(rk uint32) float64 {
	var f float64
	if rk&0x02 != 0 {
		f = float64(int32(rk) >> 2)
	} else {
		f = math.Float64frombits(uint64(rk&0xFFFFFFFC) << 32)
	}
	if rk&0x01 != 0 {
		f /= 100
	}
	return f
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
