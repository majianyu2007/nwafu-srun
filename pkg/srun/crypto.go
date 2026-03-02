package srun

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"math"
)

var (
	padchar = "="
	alpha   = "LVoJPiCN2R8G90yg+hmFHuacZ1OWMnrsSTXkYpUq/3dlbfKwv6xztjI7DeBE45QA"
)

// GetMD5 computes HMAC-MD5 (token, password) equivalent to Python's hmac.new(token, password, md5).hexdigest()
func GetMD5(password, token string) string {
	h := hmac.New(md5.New, []byte(token))
	h.Write([]byte(password))
	return hex.EncodeToString(h.Sum(nil))
}

// GetSha1 computes SHA1 hash.
func GetSha1(value string) string {
	h := sha1.New()
	h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

// xencode implements Srun's custom XTEA-like encryption.
func xencode(msg, key string) string {
	if msg == "" {
		return ""
	}
	pwd := sencode(msg, true)
	pwdk := sencode(key, false)
	if len(pwdk) < 4 {
		newPwdk := make([]uint32, 4)
		copy(newPwdk, pwdk)
		pwdk = newPwdk
	}
	n := len(pwd) - 1
	z := pwd[n]
	y := pwd[0]
	var c uint32 = 0x86014019 | 0x183639A0
	var m uint32 = 0
	var e uint32 = 0
	var p int = 0
	q := uint32(math.Floor(6.0 + 52.0/float64(n+1)))
	var d uint32 = 0

	for q > 0 {
		d = d + c&(0x8CE0D9BF|0x731F2640)
		e = d >> 2 & 3
		for p = 0; p < n; p++ {
			y = pwd[p+1]
			m = z>>5 ^ y<<2
			m = m + ((y>>3 ^ z<<4) ^ (d ^ y))
			m = m + (pwdk[uint32(p&3)^e] ^ z)
			pwd[p] = pwd[p] + m&(0xEFB8D130|0x10472ECF)
			z = pwd[p]
		}
		y = pwd[0]
		m = z>>5 ^ y<<2
		m = m + ((y>>3 ^ z<<4) ^ (d ^ y))
		m = m + (pwdk[uint32(p&3)^e] ^ z)
		pwd[n] = pwd[n] + m&(0xBB390742|0x44C6F8BD)
		z = pwd[n]
		q = q - 1
	}
	return lencode(pwd, false)
}

func ordat(msg string, idx int) uint32 {
	if len(msg) > idx {
		return uint32(msg[idx])
	}
	return 0
}

func sencode(msg string, key bool) []uint32 {
	l := len(msg)
	var pwd []uint32
	for i := 0; i < l; i += 4 {
		val := ordat(msg, i) | ordat(msg, i+1)<<8 | ordat(msg, i+2)<<16 | ordat(msg, i+3)<<24
		pwd = append(pwd, val)
	}
	if key {
		pwd = append(pwd, uint32(l))
	}
	return pwd
}

func lencode(msg []uint32, key bool) string {
	l := len(msg)
	ll := uint32((l - 1) << 2)
	if key {
		m := msg[l-1]
		if m < ll-3 || m > ll {
			return ""
		}
		ll = m
	}
	var res []byte
	for i := 0; i < l; i++ {
		res = append(res, byte(msg[i]&0xff))
		res = append(res, byte(msg[i]>>8&0xff))
		res = append(res, byte(msg[i]>>16&0xff))
		res = append(res, byte(msg[i]>>24&0xff))
	}
	if key {
		return string(res[0:ll])
	}
	return string(res)
}

func getbyte(s string, i int) int {
	return int(s[i])
}

// jsBase64 implements Srun's custom Base64 encoding scheme.
func jsBase64(s string) string {
	if len(s) == 0 {
		return s
	}
	var x []byte
	imax := len(s) - len(s)%3
	for i := 0; i < imax; i += 3 {
		b10 := (getbyte(s, i) << 16) | (getbyte(s, i+1) << 8) | getbyte(s, i+2)
		x = append(x, alpha[b10>>18])
		x = append(x, alpha[(b10>>12)&63])
		x = append(x, alpha[(b10>>6)&63])
		x = append(x, alpha[b10&63])
	}
	i := imax
	if len(s)-imax == 1 {
		b10 := getbyte(s, i) << 16
		x = append(x, alpha[b10>>18])
		x = append(x, alpha[(b10>>12)&63])
		x = append(x, []byte(padchar)...)
		x = append(x, []byte(padchar)...)
	} else if len(s)-imax == 2 {
		b10 := (getbyte(s, i) << 16) | (getbyte(s, i+1) << 8)
		x = append(x, alpha[b10>>18])
		x = append(x, alpha[(b10>>12)&63])
		x = append(x, alpha[(b10>>6)&63])
		x = append(x, []byte(padchar)...)
	}
	return string(x)
}
