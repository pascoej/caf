package caf

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"

	"github.com/sirupsen/logrus"
)

type FourByteString [4]byte

var ChunkTypeAudioDescription = stringToChunkType("desc")
var ChunkTypeChannelLayout = stringToChunkType("chan")
var ChunkTypeInformation = stringToChunkType("info")
var ChunkTypeAudioData = stringToChunkType("data")
var ChunkTypePacketTable = stringToChunkType("pakt")
var ChunkTypeMidi = stringToChunkType("midi")

func stringToChunkType(str string) (result FourByteString) {
	for i, v := range str {
		result[i] = byte(v)
	}
	return
}

type FileHeader struct {
	FileType    FourByteString
	FileVersion int16
	FileFlags   int16
}

type ChunkHeader struct {
	ChunkType FourByteString
	ChunkSize int64
}

type Data struct {
	EditCount uint32
	Data      []byte
}

type AudioFormat struct {
	SampleRate        float64
	FormatID          FourByteString
	FormatFlags       uint32
	BytesPerPacket    uint32
	FramesPerPacket   uint32
	ChannelsPerPacket uint32
	BitsPerChannel    uint32
}

type PacketTableHeader struct {
	NumberPackets     int64
	NumberValidFrames int64
	PrimingFramess    int32
	RemainderFrames   int32
}

type PacketTable struct {
	Header PacketTableHeader
	Entry  []uint64
}

func encodeInt(w io.Writer, i uint64) error {
	var byts []byte
	var cur = i
	for {
		val := byte(cur & 127)
		cur = cur >> 7
		byts = append(byts, val)
		if cur == 0 {
			break
		}
	}
	for i := len(byts) - 1; i >= 0; i-- {
		var val = byts[i]
		if i > 0 {
			val = val | 0x80
		}
		if w != nil {
			if n, err := w.Write([]byte{val}); err != nil {
				return err
			} else {
				if n != 1 {
					return errors.New("error writing")
				}
			}
		}
	}
	return nil
}

func decodeInt(r *bufio.Reader) (uint64, error) {
	var res uint64 = 0
	var bytesRead = 0
	for {
		byt, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		bytesRead += 1
		res = res << 7
		res = res | uint64(byt&127)
		if byt&128 == 0 || bytesRead >= 8 {
			return res, nil
		}
	}
}

func (c *PacketTable) decode(r *bufio.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &c.Header); err != nil {
		return err
	}
	for i := 0; i < int(c.Header.NumberPackets); i++ {
		if val, err := decodeInt(r); err != nil {
			return err
		} else {
			c.Entry = append(c.Entry, val)
		}
	}
	return nil
}

func (c *PacketTable) encode(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, c.Header); err != nil {
		return err
	}
	for i := 0; i < int(c.Header.NumberPackets); i++ {
		if err := encodeInt(w, c.Entry[i]); err != nil {
			return err
		}
	}
	return nil
}

type ChannelLayout struct {
	ChannelLayoutTag          uint32
	ChannelBitmap             uint32
	NumberChannelDescriptions uint32
	Channels                  []ChannelDescription
}

type ChannelDescription struct {
	ChannelLabel uint32
	ChannelFlags uint32
	Coordinates  [3]float32
}

type Information struct {
	Key   string
	Value string
}

type UnknownContents struct {
	Data []byte
}

type Midi = []byte

type File struct {
	FileHeader FileHeader
	Chunks     []Chunk
}

func (cf *File) Decode(r io.Reader) error {
	bufferedReader := bufio.NewReader(r)
	var fileHeader FileHeader
	if err := fileHeader.Decode(bufferedReader); err != nil {
		return err
	}
	cf.FileHeader = fileHeader
	for {
		var c Chunk
		if err := c.decode(bufferedReader); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		cf.Chunks = append(cf.Chunks, c)
	}
	return nil
}

func (cf *File) Encode(w io.Writer) error {
	if err := cf.FileHeader.Encode(w); err != nil {
		return err
	}
	for _, c := range cf.Chunks {
		if err := c.Encode(w); err != nil {
			return err
		}
	}
	return nil
}

func readString(r io.Reader) (string, error) {
	var bs []byte
	var b = make([]byte, 1)
	for {
		if _, err := r.Read(b); err != nil {
			return "", err
		} else {
			bs = append(bs, b[0])
			if b[0] == 0 {
				break
			}
		}
	}
	return string(bs), nil
}

func writeString(w io.Writer, s string) error {
	byteString := []byte(s)
	_, err := w.Write(byteString)
	return err
}

func (c *Information) decode(r io.Reader) error {
	if key, err := readString(r); err != nil {
		return err
	} else {
		c.Key = key
	}
	if value, err := readString(r); err != nil {
		return err
	} else {
		c.Value = value
	}

	return nil
}

func (c *Information) encode(w io.Writer) error {
	if err := writeString(w, c.Key); err != nil {
		return err
	}
	return writeString(w, c.Value)
}

func (c *CAFStringsChunk) decode(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &c.NumEntries); err != nil {
		return err
	}
	for i := uint32(0); i < c.NumEntries; i++ {
		var info Information
		if err := info.decode(r); err != nil {
			return err
		}
		c.Strings = append(c.Strings, info)
	}
	return nil
}

func (c *CAFStringsChunk) encode(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, &c.NumEntries); err != nil {
		return err
	}
	for i := uint32(0); i < c.NumEntries; i++ {
		if err := c.Strings[i].encode(w); err != nil {
			return err
		}
	}
	return nil
}

type CAFStringsChunk struct {
	NumEntries uint32
	Strings    []Information
}

type Chunk struct {
	Header   ChunkHeader
	Contents interface{}
}

func (c *AudioFormat) decode(r io.Reader) error {
	return binary.Read(r, binary.BigEndian, c)
}

func (c *AudioFormat) encode(w io.Writer) error {
	return binary.Write(w, binary.BigEndian, c)
}

func (c *ChannelLayout) decode(r io.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &c.ChannelLayoutTag); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &c.ChannelBitmap); err != nil {
		return err
	}
	if err := binary.Read(r, binary.BigEndian, &c.NumberChannelDescriptions); err != nil {
		return err
	}
	for i := uint32(0); i < c.NumberChannelDescriptions; i++ {
		var channelDesc ChannelDescription
		if err := binary.Read(r, binary.BigEndian, &channelDesc); err != nil {
			return err
		}
		c.Channels = append(c.Channels, channelDesc)
	}
	return nil
}

func (c *ChannelLayout) encode(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, &c.ChannelLayoutTag); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, &c.ChannelBitmap); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, &c.NumberChannelDescriptions); err != nil {
		return err
	}
	for i := uint32(0); i < c.NumberChannelDescriptions; i++ {
		if err := binary.Write(w, binary.BigEndian, &c.Channels[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *Data) decode(r *bufio.Reader, h ChunkHeader) error {
	if err := binary.Read(r, binary.BigEndian, &c.EditCount); err != nil {
		return err
	}
	if h.ChunkSize == -1 {
		// read until end
		data, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}
		c.Data = data
	} else {
		dataLength := h.ChunkSize - 4 /* for edit count*/
		data, err := ioutil.ReadAll(io.LimitReader(r, dataLength))
		if err != nil {
			return err
		}
		c.Data = data
	}
	return nil
}

func (c *Data) encode(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, &c.EditCount); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, &c.Data); err != nil {
		return err
	}
	return nil
}

func (c *Chunk) decode(r *bufio.Reader) error {
	if err := binary.Read(r, binary.BigEndian, &c.Header); err != nil {
		return err
	}
	switch c.Header.ChunkType {
	case ChunkTypeAudioDescription:
		{
			var cc AudioFormat
			if err := cc.decode(r); err != nil {
				return err
			}
			c.Contents = &cc
			break
		}
	case ChunkTypeChannelLayout:
		{
			var cc ChannelLayout
			if err := cc.decode(r); err != nil {
				return err
			}
			c.Contents = &cc
			break
		}
	case ChunkTypeInformation:
		{
			var cc CAFStringsChunk
			if err := cc.decode(r); err != nil {
				return err
			}
			c.Contents = &cc
			break
		}
	case ChunkTypeAudioData:
		{
			var cc Data
			if err := cc.decode(r, c.Header); err != nil {
				return err
			}
			c.Contents = &cc
		}
	case ChunkTypePacketTable:
		{
			var cc PacketTable
			if err := cc.decode(r); err != nil {
				return err
			}
			c.Contents = &cc
		}
	case ChunkTypeMidi:
		{
			var cc Midi
			ba := make([]byte, c.Header.ChunkSize)
			if err := binary.Read(r, binary.BigEndian, &ba); err != nil {
				return err
			}
			cc = ba
			c.Contents = cc
		}
	default:
		{
			logrus.Debugf("Got unknown chunk type")
			ba := make([]byte, c.Header.ChunkSize)
			if err := binary.Read(r, binary.BigEndian, &ba); err != nil {
				return err
			}
			c.Contents = &UnknownContents{Data: ba}
		}
	}
	return nil
}

func (c *Chunk) Encode(w io.Writer) error {
	if err := binary.Write(w, binary.BigEndian, &c.Header); err != nil {
		return err
	}
	switch c.Header.ChunkType {
	case ChunkTypeAudioDescription:
		{
			cc := c.Contents.(*AudioFormat)
			if err := cc.encode(w); err != nil {
				return err
			}
			break
		}
	case ChunkTypeChannelLayout:
		{
			cc := c.Contents.(*ChannelLayout)
			if err := cc.encode(w); err != nil {
				return err
			}
			break
		}
	case ChunkTypeInformation:
		{
			cc := c.Contents.(*CAFStringsChunk)
			if err := cc.encode(w); err != nil {
				return err
			}
			break
		}
	case ChunkTypeAudioData:
		{
			cc := c.Contents.(*Data)
			if err := cc.encode(w); err != nil {
				return err
			}
			c.Contents = &cc
		}
	case ChunkTypePacketTable:
		{
			cc := c.Contents.(*PacketTable)
			if err := cc.encode(w); err != nil {
				return err
			}
			c.Contents = &cc
		}
	case ChunkTypeMidi:
		{
			midi := c.Contents.(Midi)
			if _, err := w.Write(midi); err != nil {
				return err
			}

		}
	default:
		{
			data := c.Contents.(*UnknownContents).Data
			if _, err := w.Write(data); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *FileHeader) Decode(r io.Reader) error {
	err := binary.Read(r, binary.BigEndian, h)
	if err != nil {
		return err
	}
	if h.FileType != stringToChunkType("caff") {
		return errors.New("invalid caff header")
	}
	return nil
}

func (h *FileHeader) Encode(w io.Writer) error {
	err := binary.Write(w, binary.BigEndian, h)
	if err != nil {
		return err
	}
	return nil
}
