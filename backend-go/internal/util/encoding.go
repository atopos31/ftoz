package util

import (
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
	"strings"
)

var encodingMap = map[string]encoding.Encoding{
	"utf-8":       unicode.UTF8,
	"utf8":        unicode.UTF8,
	"utf-16":      unicode.UTF16(unicode.LittleEndian, unicode.UseBOM),
	"utf16":       unicode.UTF16(unicode.LittleEndian, unicode.UseBOM),
	"utf-16le":    unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM),
	"utf-16be":    unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM),
	"gbk":         simplifiedchinese.GBK,
	"gb2312":      simplifiedchinese.HZGB2312,
	"gb18030":     simplifiedchinese.GB18030,
	"big5":        traditionalchinese.Big5,
	"shift_jis":   japanese.ShiftJIS,
	"shiftjis":    japanese.ShiftJIS,
	"euc-jp":      japanese.EUCJP,
	"eucjp":       japanese.EUCJP,
	"iso-2022-jp": japanese.ISO2022JP,
	"euc-kr":      korean.EUCKR,
	"euckr":       korean.EUCKR,
	"iso-8859-1":  charmap.ISO8859_1,
	"latin1":      charmap.ISO8859_1,
	"windows-1252": charmap.Windows1252,
}

// GetEncoding 根据编码名称获取编码器
func GetEncoding(name string) encoding.Encoding {
	name = strings.ToLower(strings.TrimSpace(name))
	if enc, ok := encodingMap[name]; ok {
		return enc
	}
	return nil
}

// EncodeString 将 UTF-8 字符串转换为指定编码
func EncodeString(s string, encodingName string) ([]byte, error) {
	if encodingName == "" || strings.ToLower(encodingName) == "utf-8" || strings.ToLower(encodingName) == "utf8" {
		return []byte(s), nil
	}

	enc := GetEncoding(encodingName)
	if enc == nil {
		// 未知编码，返回原始 UTF-8
		return []byte(s), nil
	}

	return enc.NewEncoder().Bytes([]byte(s))
}
