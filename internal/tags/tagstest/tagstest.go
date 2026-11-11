// Package tagstest writes ID3 tags for tests, as id3.org's specifications
// lay them out (ID3v2.2, 2.3, 2.4 and ID3v1), so that tests make their
// tagged files themselves instead of keeping other programs' files.
package tagstest

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"unicode/utf16"
)

// Frame is one frame: its ID (three letters for ID3v2.2) and contents.
type Frame struct {
	ID   string
	Data []byte
}

// Text is a text frame's contents in the version's usual encoding:
// ISO-8859-1 in 2.2, UTF-16 with a byte order mark in 2.3, UTF-8 in 2.4.
func Text(major int, value string) []byte {
	switch major {
	case 2:
		return append([]byte{0}, latin1(value)...)
	case 3:
		return append([]byte{1}, utf16BOM(value)...)
	}
	return append([]byte{3}, value...)
}

// Picture is an APIC frame's contents (PIC's in 2.2): a front cover.
func Picture(major int, mime string, data []byte) []byte {
	var b bytes.Buffer
	b.WriteByte(0) // ISO-8859-1 description
	if major == 2 {
		b.WriteString("PNG")
		if mime == "image/jpeg" {
			b.Truncate(1)
			b.WriteString("JPG")
		}
	} else {
		b.WriteString(mime)
		b.WriteByte(0)
	}
	b.WriteByte(3) // the front cover
	b.WriteString("cover")
	b.WriteByte(0)
	b.Write(data)
	return b.Bytes()
}

// V2 is an ID3v2 tag of major version 2, 3 or 4 holding frames.
func V2(major int, frames ...Frame) []byte {
	var body bytes.Buffer
	for _, f := range frames {
		size := len(f.Data)
		switch major {
		case 2:
			body.WriteString(f.ID)
			body.Write([]byte{byte(size >> 16), byte(size >> 8), byte(size)})
		case 3:
			body.WriteString(f.ID)
			body.Write(binary.BigEndian.AppendUint32(nil, uint32(size)))
			body.Write([]byte{0, 0})
		default:
			body.WriteString(f.ID)
			body.Write(syncsafe(size))
			body.Write([]byte{0, 0})
		}
		body.Write(f.Data)
	}
	head := append([]byte("ID3"), byte(major), 0, 0)
	head = append(head, syncsafe(body.Len())...)
	return append(head, body.Bytes()...)
}

// V1 is an ID3v1 tag: the 128 bytes at a file's end.
func V1(title, artist, album string) []byte {
	b := make([]byte, 128)
	copy(b, "TAG")
	copy(b[3:33], title)
	copy(b[33:63], artist)
	copy(b[63:93], album)
	b[127] = 255 // no genre
	return b
}

// PNG is a w × h picture of one colour, encoded.
func PNG(w, h int, c color.NRGBA) []byte {
	picture := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(picture.Pix); i += 4 {
		picture.Pix[i], picture.Pix[i+1], picture.Pix[i+2], picture.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	var b bytes.Buffer
	if err := png.Encode(&b, picture); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func syncsafe(n int) []byte {
	return []byte{byte(n >> 21 & 0x7f), byte(n >> 14 & 0x7f), byte(n >> 7 & 0x7f), byte(n & 0x7f)}
}

func latin1(s string) []byte {
	var b []byte
	for _, r := range s {
		if r > 255 {
			r = '?'
		}
		b = append(b, byte(r))
	}
	return b
}

func utf16BOM(s string) []byte {
	b := []byte{0xff, 0xfe}
	for _, u := range utf16.Encode([]rune(s)) {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return b
}
