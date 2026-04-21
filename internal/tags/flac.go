package tags

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/go-flac/go-flac"
	"github.com/go-flac/flacpicture"
)

type flacMetadata struct {
	file   *flac.File
	comments map[string]string
	path   string
}

// ParseFLAC abre un archivo FLAC saltando posibles tags ID3v2 al inicio
func ParseFLAC(path string) (*flac.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, 10)
	if _, err := io.ReadFull(f, header); err != nil {
		return nil, err
	}

	var offset int64
	if string(header[:3]) == "ID3" {
		// El tamaño en ID3v2 es un entero "synchsafe" (7 bits por byte)
		size := int64(header[6])<<21 | int64(header[7])<<14 | int64(header[8])<<7 | int64(header[9])
		offset = 10 + size
		if (header[5]&0x10) != 0 { // Flag de footer
			offset += 10
		}
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}

	return flac.ParseBytes(f)
}

func OpenFLAC(path string) (Metadata, error) {
	f, err := ParseFLAC(path)
	if err != nil { return nil, err }
	
	m := &flacMetadata{file: f, comments: make(map[string]string), path: path}
	
	for _, block := range f.Meta {
		if block.Type == flac.VorbisComment {
			m.parseVorbis(block.Data)
			break
		}
	}
	
	return m, nil
}

// parseVorbis decodifica el bloque crudo de Vorbis Comments (FLAC)
func (f *flacMetadata) parseVorbis(data []byte) {
	reader := bytes.NewReader(data)
	
	var vendorLen uint32
	binary.Read(reader, binary.LittleEndian, &vendorLen)
	vendor := make([]byte, vendorLen)
	reader.Read(vendor)
	
	var listLen uint32
	binary.Read(reader, binary.LittleEndian, &listLen)
	
	for i := uint32(0); i < listLen; i++ {
		var commentLen uint32
		if err := binary.Read(reader, binary.LittleEndian, &commentLen); err != nil {
			break
		}
		
		// 🛡️ ESCUDO ANTI-SPIKES: Valida que la longitud tiene sentido.
		// Si el tag dice que pesa más que los bytes que nos quedan por leer o es 0, está corrupto.
		if int(commentLen) > reader.Len() || commentLen == 0 {
			break // Evitamos asignar gigabytes de memoria por error
		}
		
		comment := make([]byte, commentLen)
		reader.Read(comment)
		
		parts := strings.SplitN(string(comment), "=", 2)
		if len(parts) == 2 {
			f.comments[strings.ToUpper(parts[0])] = parts[1]
		}
	}
}

func (f *flacMetadata) Title() string       { return f.comments["TITLE"] }
func (f *flacMetadata) Artist() string      { return f.comments["ARTIST"] }
func (f *flacMetadata) Album() string       { return f.comments["ALBUM"] }
func (f *flacMetadata) Genre() string       { return f.comments["GENRE"] }
func (f *flacMetadata) Composer() string    { return f.comments["COMPOSER"] }
func (f *flacMetadata) Comment() string     { return f.comments["COMMENT"] }
func (f *flacMetadata) AlbumArtist() string { return f.comments["ALBUMARTIST"] }
func (f *flacMetadata) Year() int           { y, _ := strconv.Atoi(f.comments["DATE"]); return y }

func (f *flacMetadata) Track() (int, int) {
	n, _ := strconv.Atoi(f.comments["TRACKNUMBER"])
	t, _ := strconv.Atoi(f.comments["TRACKTOTAL"])
	return n, t
}

func (f *flacMetadata) Disc() (int, int) {
	n, _ := strconv.Atoi(f.comments["DISCNUMBER"])
	t, _ := strconv.Atoi(f.comments["DISCTOTAL"])
	return n, t
}

func (f *flacMetadata) Picture() *Picture {
	for _, block := range f.file.Meta {
		if block.Type == flac.Picture {
			pb, err := flacpicture.ParseFromMetaDataBlock(*block)
			if err != nil {
				continue
			}
			return &Picture{
				MIME:        pb.MIME,
				Data:        pb.ImageData,
				Description: pb.Description,
			}
		}
	}
	return nil
}

// Escritura (Placeholder funcional)
func (f *flacMetadata) SetTitle(s string)       { f.comments["TITLE"] = s }
func (f *flacMetadata) SetArtist(s string)      { f.comments["ARTIST"] = s }
func (f *flacMetadata) SetAlbum(s string)       { f.comments["ALBUM"] = s }
func (f *flacMetadata) SetGenre(s string)       { f.comments["GENRE"] = s }
func (f *flacMetadata) SetComposer(s string)    { f.comments["COMPOSER"] = s }
func (f *flacMetadata) SetComment(s string)     { f.comments["COMMENT"] = s }
func (f *flacMetadata) SetAlbumArtist(s string) { f.comments["ALBUMARTIST"] = s }
func (f *flacMetadata) SetYear(year int)        { f.comments["DATE"] = strconv.Itoa(year) }
func (f *flacMetadata) SetTrack(n, t int)       { f.comments["TRACKNUMBER"] = strconv.Itoa(n); f.comments["TRACKTOTAL"] = strconv.Itoa(t) }
func (f *flacMetadata) SetDisc(n, t int)        { f.comments["DISCNUMBER"] = strconv.Itoa(n); f.comments["DISCTOTAL"] = strconv.Itoa(t) }

func (f *flacMetadata) Save() error {
	// Reconstruir el bloque VorbisComment
	buf := new(bytes.Buffer)
	vendor := "sgound-engine"
	binary.Write(buf, binary.LittleEndian, uint32(len(vendor)))
	buf.WriteString(vendor)
	
	binary.Write(buf, binary.LittleEndian, uint32(len(f.comments)))
	for k, v := range f.comments {
		entry := k + "=" + v
		binary.Write(buf, binary.LittleEndian, uint32(len(entry)))
		buf.WriteString(entry)
	}
	
	newBlock := &flac.MetaDataBlock{
		Type: flac.VorbisComment,
		Data: buf.Bytes(),
	}
	
	found := false
	for i, b := range f.file.Meta {
		if b.Type == flac.VorbisComment {
			f.file.Meta[i] = newBlock
			found = true
			break
		}
	}
	if !found { f.file.Meta = append(f.file.Meta, newBlock) }
	
	return f.file.Save(f.path)
}

func (f *flacMetadata) Close() error { return nil }
