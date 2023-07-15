//go:generate goversioninfo -icon=icon.ico -manifest=main.exe.manifest
package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	mqtt "github.com/mochi-co/mqtt/server"
	"github.com/mochi-co/mqtt/server/events"
	"github.com/mochi-co/mqtt/server/listeners"
	"github.com/mochi-co/mqtt/server/listeners/auth"
)

var (
	language      string
	timeFormat    string = "2006-01-02 15:04:05"
	logFile       string
	userFile      string
	logData       string
	logStatus     string
	logFileE      bool = false
	logDataE      bool = false
	logStatusE    bool = false
	fileTimestamp bool = false
	logFileF      *os.File
	logDataF      *os.File
	logStatusF    *os.File
	monochrome    bool = false
)

func main() {
	go http.ListenAndServe(":9999", nil)
	var (
		version      string = "1.3.2"
		versionView  bool   = false
		listen       string
		onlyID       string
		onlyTopic    string
		onlyPayload  string
		onlyIdE      bool     = false
		onlyTopicE   bool     = false
		onlyPayloadE bool     = false
		onlyIdS      []string = []string{}
		onlyTopicS   []string = []string{}
		onlyPayloadS []string = []string{}
		certCA       string
		certCert     string
		certKey      string
		certPassword string
	)
	// 初始化启动参数
	flag.BoolVar(&versionView, "v", false, "Print version info")
	flag.StringVar(&language, "l", "en", "Language ( en(default) | cn )")
	flag.StringVar(&listen, "p", "127.0.0.1:1883", "Define listening on IP:Port (default: 127.0.0.1:1883 )")
	flag.StringVar(&onlyID, "c", "", "Only allow these client IDs (comma separated)")
	flag.StringVar(&onlyTopic, "t", "", "Only allow these topics (comma separated)")
	flag.StringVar(&onlyPayload, "w", "", "Only allow these words in message content (comma separated)")
	flag.StringVar(&logData, "m", "", "Log message to csv file")
	flag.StringVar(&logStatus, "s", "", "Log state changes to a csv file")
	flag.StringVar(&logFile, "o", "", "Save log to txt/log file")
	flag.BoolVar(&fileTimestamp, "ts", false, "Use timestamps in logged files")
	flag.BoolVar(&monochrome, "n", false, "Use a monochrome color scheme (When an abnormal character appears in Windows cmd.exe)")
	flag.StringVar(&userFile, "u", "", "Users and permissions file (.json, visit README.md) path")
	flag.StringVar(&certCA, "ca", "", "CA certificate file path")
	flag.StringVar(&certCert, "ce", "", "Server certificate file path")
	flag.StringVar(&certKey, "ck", "", "Server key file path")
	flag.StringVar(&certPassword, "cp", "", "Server key file password")
	flag.Parse()
	logPrint("I", lang("TITLE")+" v"+version+" (KagurazakaYashi@Tongdy, 2023)")
	logPrint("I", lang("HELP")+" https://github.com/tongdysoft/mqtt-test-server")
	// 初始化设置
	if versionView {
		return
	}
	if len(onlyID) > 0 {
		onlyIdS = strings.Split(onlyID, ",")
		logPrint("C", fmt.Sprintf("%s%s: %s", lang("ONLY"), lang("CLIENT"), onlyIdS))
		onlyIdE = true
	}
	if len(onlyTopic) > 0 {
		onlyTopicS = strings.Split(onlyTopic, ",")
		logPrint("C", fmt.Sprintf("%s%s: %s", lang("ONLY"), lang("TOPIC"), onlyTopicS))
		onlyTopicE = true
	}
	if len(onlyPayload) > 0 {
		onlyPayloadS = strings.Split(onlyPayload, ",")
		logPrint("C", fmt.Sprintf("%s%s: %s", lang("ONLY"), lang("WORD"), onlyPayloadS))
		onlyPayloadE = true
	}
	logInit(listen, lang("TITLE")+" v"+version)
	// 监听结束信号
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		done <- true
		close(sigs)
	}()
	// 加载 SSL 证书
	tlsConfig := &tls.Config{}
	certPool := x509.NewCertPool()
	var useTLS bool = false
	if len(certCA) > 0 {
		contentC, err := os.ReadFile(certCA)
		if err != nil {
			logPrint("X", fmt.Sprintf("%s%s: %s: %s)", lang("CACERT"), lang("ERROR"), certCA, err.Error()))
			return
		}
		var isok bool = certPool.AppendCertsFromPEM(contentC)
		if !isok {
			logPrint("X", fmt.Sprintf("%s%s: %s (%d)", lang("CACERT"), lang("ERROR"), certCA, len(contentC)))
			return
		}
		tlsConfig = &tls.Config{
			ClientCAs:  certPool,
			ClientAuth: tls.RequireAndVerifyClientCert,
		}
		logPrint("C", fmt.Sprintf("%s: %s (%d)", lang("CACERT"), certCA, len(contentC)))
		useTLS = true
	}
	if len(certCert) > 0 && len(certKey) == 0 {
		logPrint("X", fmt.Sprintf("%s%s: %s", lang("SERVERKEY"), lang("ERROR"), lang("NOTEMPTY")))
		return
	}
	if len(certCert) == 0 && len(certKey) > 0 {
		logPrint("X", fmt.Sprintf("%s%s: %s", lang("SERVERCERT"), lang("ERROR"), lang("NOTEMPTY")))
		return
	}
	if len(certCert) > 0 && len(certKey) > 0 {
		contentC, err := os.ReadFile(certCert)
		if err != nil {
			logPrint("X", fmt.Sprintf("%s%s: %s: %s", lang("SERVERCERT"), lang("ERROR"), certCert, err.Error()))
			return
		}
		logPrint("C", fmt.Sprintf("%s: %s (%d)", lang("SERVERCERT"), certCert, len(contentC)))
		contentK, err := os.ReadFile(certKey)
		if err != nil {
			logPrint("X", fmt.Sprintf("%s%s: %s: %s", lang("SERVERKEY"), lang("ERROR"), certKey, err.Error()))
			return
		}
		logPrint("C", fmt.Sprintf("%s: %s (%d)", lang("SERVERKEY"), certKey, len(contentK)))
		var cert tls.Certificate = LoadCert(contentC, contentK, certPassword)
		if cert.Certificate == nil {
			return
		}
		if len(certCA) > 0 {
			tlsConfig = &tls.Config{
				ClientCAs:    certPool,
				Certificates: []tls.Certificate{cert},
			}
		} else {
			tlsConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
		}
		useTLS = true
	}
	if len(certPassword) > 0 {
		logPrint("C", fmt.Sprintf("%s: (%d)", lang("SERVERKEYPWD"), len(certPassword)))
	}
	// 初始化 MQTT 服务器
	logPrint("I", lang("BOOTING")+listen+" ...")
	server := mqtt.NewServer(nil)
	tcp := listeners.NewTCP(listen, listen)
	err := error(nil)
	var conf listeners.Config = listeners.Config{}
	if useTLS {
		if len(userFile) > 0 {
			conf = listeners.Config{
				Auth:      loadUserAuthFile(userFile),
				TLSConfig: tlsConfig,
			}
		} else {
			conf = listeners.Config{
				Auth:      new(auth.Allow),
				TLSConfig: tlsConfig,
			}
		}
	} else {
		if len(userFile) > 0 {
			conf = listeners.Config{
				Auth: loadUserAuthFile(userFile),
			}
		} else {
			conf = listeners.Config{
				Auth: new(auth.Allow),
			}
		}
	}
	err = server.AddListener(tcp, &conf)
	if err != nil {
		log.Fatal(err)
	}
	// 启动 MQTT 服务器
	go func() {
		err := server.Serve()
		if err != nil {
			logPrint("X", lang("SERVERFAIL"))
			log.Fatal(err)
		}
	}()
	// server.Events.OnProcessMessage = func(cl events.Client, pk events.Packet) (pkx events.Packet, err error) {
	// 	return pkx, err
	// }
	// 设备连接出错
	server.Events.OnError = func(cl events.Client, err error) {
		logPrint("D", fmt.Sprintf("%s %s: %v", lang("CLIENT"), strCL(cl), err))
	}
	// 有设备连接到服务器
	server.Events.OnConnect = func(cl events.Client, pk events.Packet) {
		pkJsonB, err := json.Marshal(pk)
		if err != nil {
			pkJsonB = []byte("")
		}
		clJsonB, err := json.Marshal(cl)
		if err != nil {
			clJsonB = []byte("")
		}
		var infoJson string = fmt.Sprintf("{\"Client\":%s,\"Packet\":%s}", string(clJsonB), string(pkJsonB))
		logFileStr(true, strCL(cl), lang("CONNECT"), strings.ReplaceAll(infoJson, "\"", "'"))
		logPrint("L", fmt.Sprintf("%s %s %s: %s", lang("CLIENT"), strCL(cl), lang("CONNECT"), infoJson))
	}
	// 设备断开连接
	server.Events.OnDisconnect = func(cl events.Client, err error) {
		logFileStr(true, strCL(cl), lang("DISCONNECT"), strings.ReplaceAll(fmt.Sprint(err), "\n", " "))
		logPrint("D", fmt.Sprintf("%s %s %s: %v", lang("CLIENT"), strCL(cl), lang("DISCONNECT"), err))
	}
	// 收到订阅请求
	server.Events.OnSubscribe = func(filter string, cl events.Client, qos byte) {
		logFileStr(true, strCL(cl), lang("SUBSCRIBED"), fmt.Sprintf("%s (QOS%d)", filter, qos))
		logPrint("S", fmt.Sprintf("%s %s %s %s, (QOS:%v)", lang("CLIENT"), strCL(cl), lang("SUBSCRIBED"), filter, qos))
	}
	// 收到取消订阅请求
	server.Events.OnUnsubscribe = func(filter string, cl events.Client) {
		logFileStr(true, strCL(cl), lang("UNSUBSCRIBED"), filter)
		logPrint("U", fmt.Sprintf("%s %s %s %s", lang("CLIENT"), strCL(cl), lang("UNSUBSCRIBED"), filter))
	}
	// 收到消息
	server.Events.OnMessage = func(cl events.Client, pk events.Packet) (pkx events.Packet, err error) {
		pkx = pk
		var clID *string = &cl.ID
		if onlyIdE && !in(&onlyIdS, clID) {
			return
		}
		var topic *string = &pkx.TopicName
		if onlyTopicE && !in(&onlyTopicS, topic) {
			return
		}
		var payload string = string(pkx.Payload)
		if onlyPayloadE {
			var inWord bool = false
			for _, word := range onlyPayloadS {
				if strings.Contains(payload, word) {
					inWord = true
					break
				}
			}
			if !inWord {
				return
			}
		}
		logFileStr(false, strCL(cl), *topic, payload)
		logPrint("M", fmt.Sprintf("%s: %s, %s: %s, %s: %s", lang("MESSAGE"), strCL(cl), lang("TOPIC"), *topic, lang("PAYLOAD"), payload)) // 收到消息，发件人...
		return pk, nil
	}
	// 启动完毕
	logPrint("I", lang("BOOTOK"))
	// 处理结束信号
	<-done
	close(done)
	logPrint("X", lang("NEEDSTOP"))
	server.Close()
	if logFileE {
		logFileF.Close()
	}
	if logDataE {
		logDataF.Close()
	}
	if logStatusE {
		logStatusF.Close()
	}
	logPrint("I", lang("END"))
	os.Exit(0)
}

func in(strArr *[]string, str *string) bool {
	sort.Strings(*strArr)
	index := sort.SearchStrings(*strArr, *str)
	if index < len(*strArr) && (*strArr)[index] == *str {
		return true
	}
	return false
}
