package config

import (
	log "github.com/sirupsen/logrus"
	"net/url"
	"os"
)

type serverTOML struct {
	Port     int    `mapstructure:"port"`
	Pretend  string `mapstructure:"pretend"`
	Secure   bool   `mapstructure:"secure"`
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type serverGo struct {
	Port     uint16
	Pretend  *url.URL
	Secure   bool
	CertFile os.FileInfo
	KeyFile  os.FileInfo
}

func (st *serverTOML) Init() (sg *serverGo, err error) {
	sg = &serverGo{
		Port:   uint16(st.Port),
		Secure: st.Secure,
	}
	sg.Pretend, err = url.Parse(st.Pretend)
	if err != nil {
		log.WithField("server.pretend", st.Pretend).Error(err)
		return nil, err
	}
	if sg.Secure {
		sg.CertFile, err = os.Stat(st.CertFile)
		if err != nil {
			log.WithField("server.cert_file", st.CertFile).Error(err)
			return nil, err
		}
		sg.KeyFile, err = os.Stat(st.KeyFile)
		if err != nil {
			log.WithField("server.key_file", st.KeyFile).Error(err)
			return nil, err
		}
	}
	return sg, nil
}
