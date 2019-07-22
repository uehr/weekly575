package data

import (
	"os"
	"time"
)


func dicUniUniDic001Bytes() ([]byte, error) {
	return bindataRead(
		_dicUniUniDic001,
		"dic/uni/uni.dic.001",
	)
}

func dicUniUniDic001() (*asset, error) {
	bytes, err := dicUniUniDic001Bytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "dic/uni/uni.dic.001", size: 10485760, mode: os.FileMode(420), modTime: time.Unix(1555398884, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}