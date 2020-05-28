// Copyright 2016 - 2020 The excelize Authors. All rights reserved. Use of
// this source code is governed by a BSD-style license that can be found in
// the LICENSE file.
//
// Package excelize providing a set of functions that allow you to write to
// and read from XLSX files. Support reads and writes XLSX file generated by
// Microsoft Excel™ 2007 and later. Support save file without losing original
// charts of XLSX. This library needs Go version 1.10 or later.

package excelize

import (
	"archive/zip"
	"bytes"
	"container/list"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unsafe"
)

// ReadZipReader can be used to read the spreadsheet in memory without touching the
// filesystem.
func ReadZipReader(r *zip.Reader) (map[string][]byte, int, error) {
	var err error
	fileList := make(map[string][]byte, len(r.File))
	worksheets := 0
	for _, v := range r.File {
		if fileList[v.Name], err = readFile(v); err != nil {
			return nil, 0, err
		}
		if strings.HasPrefix(v.Name, "xl/worksheets/sheet") {
			worksheets++
		}
	}
	return fileList, worksheets, nil
}

// readXML provides a function to read XML content as string.
func (f *File) readXML(name string) []byte {
	if content, ok := f.XLSX[name]; ok {
		return content
	}
	return []byte{}
}

// saveFileList provides a function to update given file content in file list
// of XLSX.
func (f *File) saveFileList(name string, content []byte) {
	newContent := make([]byte, 0, len(XMLHeader)+len(content))
	newContent = append(newContent, []byte(XMLHeader)...)
	newContent = append(newContent, content...)
	f.XLSX[name] = newContent
}

// Read file content as string in a archive file.
func readFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	dat := make([]byte, 0, file.FileInfo().Size())
	buff := bytes.NewBuffer(dat)
	_, _ = io.Copy(buff, rc)
	rc.Close()
	return buff.Bytes(), nil
}

// SplitCellName splits cell name to column name and row number.
//
// Example:
//
//     excelize.SplitCellName("AK74") // return "AK", 74, nil
//
func SplitCellName(cell string) (string, int, error) {
	alpha := func(r rune) bool {
		return ('A' <= r && r <= 'Z') || ('a' <= r && r <= 'z')
	}

	if strings.IndexFunc(cell, alpha) == 0 {
		i := strings.LastIndexFunc(cell, alpha)
		if i >= 0 && i < len(cell)-1 {
			col, rowstr := cell[:i+1], cell[i+1:]
			if row, err := strconv.Atoi(rowstr); err == nil && row > 0 {
				return col, row, nil
			}
		}
	}
	return "", -1, newInvalidCellNameError(cell)
}

// JoinCellName joins cell name from column name and row number.
func JoinCellName(col string, row int) (string, error) {
	normCol := strings.Map(func(rune rune) rune {
		switch {
		case 'A' <= rune && rune <= 'Z':
			return rune
		case 'a' <= rune && rune <= 'z':
			return rune - 32
		}
		return -1
	}, col)
	if len(col) == 0 || len(col) != len(normCol) {
		return "", newInvalidColumnNameError(col)
	}
	if row < 1 {
		return "", newInvalidRowNumberError(row)
	}
	return normCol + strconv.Itoa(row), nil
}

// ColumnNameToNumber provides a function to convert Excel sheet column name
// to int. Column name case insensitive. The function returns an error if
// column name incorrect.
//
// Example:
//
//     excelize.ColumnNameToNumber("AK") // returns 37, nil
//
func ColumnNameToNumber(name string) (int, error) {
	if len(name) == 0 {
		return -1, newInvalidColumnNameError(name)
	}
	col := 0
	multi := 1
	for i := len(name) - 1; i >= 0; i-- {
		r := name[i]
		if r >= 'A' && r <= 'Z' {
			col += int(r-'A'+1) * multi
		} else if r >= 'a' && r <= 'z' {
			col += int(r-'a'+1) * multi
		} else {
			return -1, newInvalidColumnNameError(name)
		}
		multi *= 26
	}
	if col > TotalColumns {
		return -1, fmt.Errorf("column number exceeds maximum limit")
	}
	return col, nil
}

// ColumnNumberToName provides a function to convert the integer to Excel
// sheet column title.
//
// Example:
//
//     excelize.ColumnNumberToName(37) // returns "AK", nil
//
func ColumnNumberToName(num int) (string, error) {
	if num < 1 {
		return "", fmt.Errorf("incorrect column number %d", num)
	}
	var col string
	for num > 0 {
		col = string((num-1)%26+65) + col
		num = (num - 1) / 26
	}
	return col, nil
}

// CellNameToCoordinates converts alphanumeric cell name to [X, Y] coordinates
// or returns an error.
//
// Example:
//
//    excelize.CellNameToCoordinates("A1") // returns 1, 1, nil
//    excelize.CellNameToCoordinates("Z3") // returns 26, 3, nil
//
func CellNameToCoordinates(cell string) (int, int, error) {
	const msg = "cannot convert cell %q to coordinates: %v"

	colname, row, err := SplitCellName(cell)
	if err != nil {
		return -1, -1, fmt.Errorf(msg, cell, err)
	}
	if row > TotalRows {
		return -1, -1, fmt.Errorf("row number exceeds maximum limit")
	}
	col, err := ColumnNameToNumber(colname)
	return col, row, err
}

// CoordinatesToCellName converts [X, Y] coordinates to alpha-numeric cell
// name or returns an error.
//
// Example:
//
//    excelize.CoordinatesToCellName(1, 1) // returns "A1", nil
//
func CoordinatesToCellName(col, row int) (string, error) {
	if col < 1 || row < 1 {
		return "", fmt.Errorf("invalid cell coordinates [%d, %d]", col, row)
	}
	colname, err := ColumnNumberToName(col)
	return fmt.Sprintf("%s%d", colname, row), err
}

// boolPtr returns a pointer to a bool with the given value.
func boolPtr(b bool) *bool { return &b }

// intPtr returns a pointer to a int with the given value.
func intPtr(i int) *int { return &i }

// float64Ptr returns a pofloat64er to a float64 with the given value.
func float64Ptr(f float64) *float64 { return &f }

// stringPtr returns a pointer to a string with the given value.
func stringPtr(s string) *string { return &s }

// defaultTrue returns true if b is nil, or the pointed value.
func defaultTrue(b *bool) bool {
	if b == nil {
		return true
	}
	return *b
}

// parseFormatSet provides a method to convert format string to []byte and
// handle empty string.
func parseFormatSet(formatSet string) []byte {
	if formatSet != "" {
		return []byte(formatSet)
	}
	return []byte("{}")
}

// namespaceStrictToTransitional provides a method to convert Strict and
// Transitional namespaces.
func namespaceStrictToTransitional(content []byte) []byte {
	var namespaceTranslationDic = map[string]string{
		StrictSourceRelationship:         SourceRelationship,
		StrictSourceRelationshipChart:    SourceRelationshipChart,
		StrictSourceRelationshipComments: SourceRelationshipComments,
		StrictSourceRelationshipImage:    SourceRelationshipImage,
		StrictNameSpaceSpreadSheet:       NameSpaceSpreadSheet,
	}
	for s, n := range namespaceTranslationDic {
		content = bytesReplace(content, stringToBytes(s), stringToBytes(n), -1)
	}
	return content
}

// stringToBytes cast a string to bytes pointer and assign the value of this
// pointer.
func stringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&s))
}

// bytesReplace replace old bytes with given new.
func bytesReplace(s, old, new []byte, n int) []byte {
	if n == 0 {
		return s
	}

	if len(old) < len(new) {
		return bytes.Replace(s, old, new, n)
	}

	if n < 0 {
		n = len(s)
	}

	var wid, i, j, w int
	for i, j = 0, 0; i < len(s) && j < n; j++ {
		wid = bytes.Index(s[i:], old)
		if wid < 0 {
			break
		}

		w += copy(s[w:], s[i:i+wid])
		w += copy(s[w:], new)
		i += wid + len(old)
	}

	w += copy(s[w:], s[i:])
	return s[0:w]
}

// genSheetPasswd provides a method to generate password for worksheet
// protection by given plaintext. When an Excel sheet is being protected with
// a password, a 16-bit (two byte) long hash is generated. To verify a
// password, it is compared to the hash. Obviously, if the input data volume
// is great, numerous passwords will match the same hash. Here is the
// algorithm to create the hash value:
//
// take the ASCII values of all characters shift left the first character 1 bit,
// the second 2 bits and so on (use only the lower 15 bits and rotate all higher bits,
// the highest bit of the 16-bit value is always 0 [signed short])
// XOR all these values
// XOR the count of characters
// XOR the constant 0xCE4B
func genSheetPasswd(plaintext string) string {
	var password int64 = 0x0000
	var charPos uint = 1
	for _, v := range plaintext {
		value := int64(v) << charPos
		charPos++
		rotatedBits := value >> 15 // rotated bits beyond bit 15
		value &= 0x7fff            // first 15 bits
		password ^= (value | rotatedBits)
	}
	password ^= int64(len(plaintext))
	password ^= 0xCE4B
	return strings.ToUpper(strconv.FormatInt(password, 16))
}

// Stack defined an abstract data type that serves as a collection of elements.
type Stack struct {
	list *list.List
}

// NewStack create a new stack.
func NewStack() *Stack {
	list := list.New()
	return &Stack{list}
}

// Push a value onto the top of the stack.
func (stack *Stack) Push(value interface{}) {
	stack.list.PushBack(value)
}

// Pop the top item of the stack and return it.
func (stack *Stack) Pop() interface{} {
	e := stack.list.Back()
	if e != nil {
		stack.list.Remove(e)
		return e.Value
	}
	return nil
}

// Peek view the top item on the stack.
func (stack *Stack) Peek() interface{} {
	e := stack.list.Back()
	if e != nil {
		return e.Value
	}
	return nil
}

// Len return the number of items in the stack.
func (stack *Stack) Len() int {
	return stack.list.Len()
}

// Empty the stack.
func (stack *Stack) Empty() bool {
	return stack.list.Len() == 0
}
