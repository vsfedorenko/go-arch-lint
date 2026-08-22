package code

import (
	"bufio"
	"bytes"
	"io"
	"os"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/vsfedorenko/go-arch-lint/v3/internal/models/domain"
)

func readFile(fileName string) (content io.Reader, linesCount int) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, 0
	}

	linesCount, err = lineCounter(file)
	if err != nil {
		return nil, 0
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, 0
	}

	return file, linesCount
}

func readLines(r io.Reader, ref domain.Reference) []byte {
	sc := bufio.NewScanner(r)
	currentLine := 0
	var buffer bytes.Buffer

	for sc.Scan() {
		currentLine++

		if currentLine >= ref.LineFrom && currentLine <= ref.LineTo {
			buffer.Write(sc.Bytes())

			if currentLine != ref.LineTo {
				buffer.WriteByte('\n')
			}
		}
	}

	return buffer.Bytes()
}

func highlightContent(filePath string, code []byte) []byte {
	lexer := lexers.Match(filePath)
	if lexer == nil {
		lexer = lexers.Fallback
	}

	style := styles.Get("trac")
	formatter := formatters.TTY8

	iterator, err := lexer.Tokenise(nil, string(code))
	if err != nil {
		return []byte{}
	}

	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return []byte{}
	}

	return buf.Bytes()
}
