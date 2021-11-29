package main

import (
	"io"
	"os"

	"github.com/BurntSushi/toml"
)

type GameConf struct {
	Sizes  []uint `toml:"sizes"`
	Stones []uint `toml:"stones"`
}

type WSConf struct {
	Enabled bool `toml:"enabled"`
}

type WebConf struct {
	Host string `toml:"host"`
	Port uint   `toml:"port"`
}

type TCPConf struct {
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    uint   `toml:"port"`
	Ping    bool   `toml:"ping"`
	Timeout uint   `time:"timeout"`
	Retries uint   `toml:"retries"`
}

type DBConf struct {
	File    string `toml:"file"`
	Threads uint   `toml:""`
	Mode    string `toml:"mode"`
}

type Conf struct {
	Debug    bool     `toml:"debug"`
	Endless  bool     `toml:"endless"`
	Database DBConf   `toml:"database"`
	Game     GameConf `toml:"game"`
	Web      WebConf  `toml:"web"`
	WS       WSConf   `toml:"websocket"`
	TCP      TCPConf  `toml:"tcp"`
}

var defaultConfig = Conf{
	Debug: false,
	Database: DBConf{
		File:    "kalah.sql",
		Threads: 1,
		Mode:    "rwc",
	},
	Endless: true,
	Game: GameConf{
		Sizes:  []uint{4, 5, 6, 7, 8, 9, 10, 11, 12},
		Stones: []uint{4, 5, 6, 7, 8, 9, 10, 11, 12},
	},
	Web: WebConf{
		Host: "0.0.0.0",
		Port: 8080,
	},
	WS: WSConf{
		Enabled: false,
	},
	TCP: TCPConf{
		Enabled: true,
		Host:    "0.0.0.0",
		Port:    2671,
		Ping:    true,
		Timeout: 20,
		Retries: 8,
	},
}

func parseConf(r io.Reader) (*Conf, error) {
	var conf Conf
	_, err := toml.NewDecoder(r).Decode(conf)
	return &conf, err
}

func openConf(name string) (*Conf, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return parseConf(file)
}
