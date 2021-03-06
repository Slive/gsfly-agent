/*
 * Author:slive
 * DATE:2020/9/10
 */
package config

import (
	"fmt"
	"github.com/slive/gsfly-agent/agent"
	"github.com/slive/gsfly/channel"
	logx "github.com/slive/gsfly/logger"
	"github.com/slive/gsfly/socket"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var logDirKey = "agent.log.dir"
var logFileKey = "agent.log.file"
var logLevelKey = "agent.log.level"
var serverIdKey = "agent.server.id"

func InitServiceConf(config map[string]string) []agent.IServiceConf {
	logDir := config[logDirKey]
	delete(config, logDirKey)

	logFile := config[logFileKey]
	delete(config, logFileKey)

	logLevel := config[logLevelKey]
	delete(config, logLevelKey)

	initLogConf(logFile, logDir, logLevel)

	readPoolConf := initReadPoolConf(config)
	logx.Info("readPoolConf:", readPoolConf)

	channelConf := initChannelConf(config)
	logx.Info("channelConf:", channelConf)

	channel.InitDefChannelConf(readPoolConf, channelConf)

	agentId := config[serverIdKey]
	if len(agentId) <= 0 {
		agentId = fmt.Sprintf("agent-%v", rand.Int())
	}

	locations := initLocations(config)

	serverConfs := initServerConf(config, agentId, channelConf)
	if len(serverConfs) <= 0 {
		logx.Panic("serverconf init is nil.")
	}
	logx.Info("serverItemConf:", serverConfs)

	upstreamConfs := initUpstreamConfs(config)
	logx.Info("upstreamConfs:", upstreamConfs)

	serviceConfs := make([]agent.IServiceConf, len(serverConfs))
	for index, sconf := range serverConfs {
		agServerConf := agent.NewAgServerConf(agentId, sconf, locations...)
		serviceConf := agent.NewServiceConf(agentId, agServerConf, upstreamConfs...)
		serviceConfs[index] = serviceConf
	}
	return serviceConfs
}

func initLogConf(logFile string, logDir string, logLevel string) {
	var logConf *logx.LogConf
	if len(logFile) > 0 {
		logConf = &logx.LogConf{
			LogFile:        logFile,
			LogDir:         logDir,
			Level:          logLevel,
			MaxRemainCount: 10,
		}
		logx.InitLogger(logConf)
		logx.Info("logConf:", logConf)
	} else {
		logx.InitDefLogger()
	}
}

var readPoolKey = "agent.readpool.maxCpuSize"
var readQueueKey = "agent.readqueue.maxSize"

func initReadPoolConf(config map[string]string) *channel.ReadPoolConf {
	readPoolStr := config[readPoolKey]
	delete(config, readPoolKey)
	readPoolSize := channel.MAX_READ_POOL_EVERY_CPU
	if len(readPoolStr) > 0 {
		retInt, err := strconv.ParseInt(readPoolStr, 10, 32)
		if err == nil {
			readPoolSize = int(retInt)
		} else {
			logx.Panic("invalid readPoolSize.")
		}
	}

	readQueueStr := config[readQueueKey]
	delete(config, readQueueKey)
	readQueueSize := channel.MAX_READ_QUEUE_SIZE
	if len(readQueueStr) > 0 {
		retInt, err := strconv.ParseInt(readQueueStr, 10, 32)
		if err == nil {
			readQueueSize = int(retInt)
		} else {
			logx.Panic("invalid readQueueSize.")
		}
	}
	readPoolConf := channel.NewReadPoolConf(runtime.NumCPU()*readPoolSize, readQueueSize)
	return readPoolConf
}

var upsPrefix = "agent.upstream."
var upsIdKey = upsPrefix + "id"

func initUpstreamConfs(config map[string]string) []agent.IUpstreamConf {
	var upstreamConfs []agent.IUpstreamConf
	upstreamMap := make(map[string]string)
	for key, v := range config {
		if strings.Contains(key, upsPrefix) {
			delete(config, key)
			upstreamMap[key] = v
		}
	}

	if len(upstreamMap) > 0 {
		upId := upstreamMap[upsIdKey]
		delete(config, upsIdKey)
		if len(upId) > 0 {
			var upsIds []string
			// 多个upstreamId，支持";"或者","分割
			if strings.Index(upId, ";") > 0 {
				upsIds = strings.Split(upId, ";")
			} else if strings.Index(upId, ",") > 0 {
				upsIds = strings.Split(upId, ",")
			} else {
				upsIds = []string{upId}
			}
			if upsIds == nil {
				logx.Panic("invalid upstreamId")
			}

			for _, upsId := range upsIds {
				upsTypeKey := upsPrefix + upsId + ".type"
				upsType := upstreamMap[upsTypeKey]
				delete(upstreamMap, upsTypeKey)

				upsLbKey := upsPrefix + upsId + ".loadBalance"
				loadBalanceStr := upstreamMap[upsLbKey]
				delete(upstreamMap, upsLbKey)

				var dstClientConfs []socket.IClientConf
				dtsKey := upsPrefix + upsId + ".dstclient"
				var loadbalance agent.LoadBalanceType
				var err error
				if len(loadBalanceStr) <= 0 {
					loadbalance = agent.LOADBALANCE_DEFAULT
				} else {
					loadbalance, err = agent.GetLoadBalanceType(loadBalanceStr)
					if err != nil {
						logx.Panic(err)
					}
				}

				dstIndex := 0
				for {
					indexKey := fmt.Sprintf((dtsKey + ".%v."), dstIndex)
					// ip不可为空
					dstIpKey := indexKey + "ip"
					dstIp := upstreamMap[dstIpKey]
					delete(upstreamMap, dstIpKey)
					if len(dstIp) <= 0 {
						break
					}

					dstPortKey := indexKey + "port"
					dstPortStr := upstreamMap[dstPortKey]
					delete(upstreamMap, dstPortKey)
					dstPort := 19980
					if len(dstPortStr) > 0 {
						retInt, err := strconv.ParseInt(dstPortStr, 10, 32)
						if err != nil {
							logx.Panic("dstPort is invalid, dstPortKey:" + dstPortKey)
						}
						dstPort = int(retInt)
					}

					dstNetworkKey := indexKey + "network"
					network := upstreamMap[dstNetworkKey]
					delete(upstreamMap, dstNetworkKey)
					var dstClientConf socket.IClientConf
					if network == channel.NETWORK_WS.String() {
						dstSchemeKey := indexKey + "scheme"
						dstScheme := upstreamMap[dstSchemeKey]
						logx.Info(dstSchemeKey + "=" + dstScheme)
						delete(upstreamMap, dstSchemeKey)

						dstPathKey := indexKey + "path"
						dstPath := upstreamMap[dstPathKey]
						delete(upstreamMap, dstPathKey)

						dstSubprotocolKey := indexKey + "subprotocol"
						dstSubrotocol := upstreamMap[dstSubprotocolKey]
						delete(upstreamMap, dstSubprotocolKey)
						dstClientConf = socket.NewWsClientConf(dstIp, dstPort, dstScheme, dstPath, dstSubrotocol)
					} else if network == channel.NETWORK_KCP.String() {
						dstClientConf = socket.NewKcpClientConf(dstIp, dstPort)
					} else if network == channel.NETWORK_TCP.String() {
						dstClientConf = socket.NewTcpClientConf(dstIp, dstPort)
					} else if network == channel.NETWORK_UDP.String() {
						dstClientConf = socket.NewUdpClientConf(dstIp, dstPort)
					} else {
						dstClientConf = socket.NewClientConf(dstIp, dstPort, channel.ToNetwork(network))
					}
					logx.Info("dstclient networkkey:", dstNetworkKey)
					logx.Info("dstClientConf:", dstClientConf)
					if dstClientConf != nil {
						if dstClientConfs == nil {
							dstClientConfs = make([]socket.IClientConf, 1)
							dstClientConfs[0] = dstClientConf
						} else {
							dstClientConfs = append(dstClientConfs, dstClientConf)
						}
					}
					dstIndex++
				}

				var upstreamConf agent.IUpstreamConf
				if (len(upsType) <= 0 || upsType == agent.UPSTREAM_PROXY) && (dstClientConfs != nil) {
					upstreamConf = agent.NewProxyConf(upsId, loadbalance, dstClientConfs...)
				} else {
					// TODO...
				}
				logx.Info("upstreamConf:", upstreamConf)
				if upstreamConf != nil {
					upstreamConfs = append(upstreamConfs, upstreamConf)
				}
			}
		} else {
			logx.Panic("upsteamId is nil.")
		}
	}
	return upstreamConfs
}

var serverKey = "agent.server"
var serverIpKey = "agent.server.ip"
var serverPortKey = "agent.server.port"
var serverNetworkKey = "agent.server.network"
var serverMaxChSizeKey = "agent.server.maxChannelSize"
var serverWsSchemeKey = "agent.server.scheme"
var serverWsKey = "agent.server.ws"
var serverWsPathKey = "agent.server.path"
var serverWsSubKey = "agent.server.subprotocol"

func initServerConf(config map[string]string, agentId string, defChannConf channel.IChannelConf) []socket.IServerConf {

	itemConfs := make([]serverItemConf, 0)
	network := config[serverNetworkKey]
	if len(network) <= 0 {
		network = channel.NETWORK_WS.String()
	}
	serverIp := config[serverIpKey]
	if len(serverIp) <= 0 {
		// 本地ip
		serverIp = ""
	}
	maxChannelSizeStr := config[serverMaxChSizeKey]

	var index = 0
	for {
		sPortKey := fmt.Sprintf("%v.%v.port", serverKey, index)
		sPortstr := config[sPortKey]
		if len(sPortstr) <= 0 {
			break
		}

		sIpKey := fmt.Sprintf("%v.%v.ip", serverKey, index)
		sIp := config[sIpKey]
		if len(sIp) <= 0 {
			// 为空则取父ip
			sIp = serverIp
		}
		sNetworkKey := fmt.Sprintf("%v.%v.network", serverKey, index)
		sNetworkstr := config[sNetworkKey]
		if len(sNetworkKey) <= 0 {
			// 为空则取父network
			sNetworkKey = network
		}

		sNetMaxChannelSizeKey := fmt.Sprintf("%v.%v.maxChannelSize", serverKey, index)
		sNetMaxChannelSizeStr := config[sNetMaxChannelSizeKey]
		if len(sNetMaxChannelSizeStr) <= 0 {
			// 为空则取父network
			sNetMaxChannelSizeStr = maxChannelSizeStr
		}
		itemConfs = newServerItemConf(sPortstr, sIp, sNetworkstr, sNetMaxChannelSizeStr, itemConfs)
		index++
	}

	portStr := config[serverPortKey]
	if len(portStr) <= 0 {
		if len(itemConfs) <= 0 {
			portStr = "9080"
		}
	}
	if len(portStr) > 0 {
		itemConfs = newServerItemConf(portStr, serverIp, network, maxChannelSizeStr, itemConfs)
	}

	sconfs := make([]socket.IServerConf, 0)
	var scheme string
	var wsConfs []socket.IServerChildConf
	for index, sc := range itemConfs {
		// 共用部分
		serverIp := sc.GetIp()
		port := sc.GetPort()
		network := sc.network
		maxChannelSize := sc.maxChannelSize
		var serverConf socket.IServerConf
		if network == channel.NETWORK_WS.String() {
			if len(wsConfs) <= 0 {
				scheme, wsConfs = initServerWsConf(config)
			}
			serverConf = socket.NewWsServerConf(serverIp, port, scheme, wsConfs...)
			serverConf.SetMaxChannelSize(maxChannelSize)
		} else if network == channel.NETWORK_KCP.String() {
			serverConf = socket.NewKcpServerConf(serverIp, port)
			serverConf.SetMaxChannelSize(maxChannelSize)
		} else if network == channel.NETWORK_TCP.String() {
			serverConf = socket.NewTcpServerConf(serverIp, port)
			serverConf.SetMaxChannelSize(maxChannelSize)
		} else {
			logx.Info("unsupport network:", network)
		}
		if serverConf != nil {
			serverConf.SetId(fmt.Sprintf("%v.%v", agentId, index))
			serverConf.CopyChConf(defChannConf)
			sconfs = append(sconfs, serverConf)
		}
	}
	return sconfs
}

func newServerItemConf(portStr string, serverIp string, network string, maxChannelSizeStr string, serverItemConfs []serverItemConf) []serverItemConf {
	if len(portStr) > 0 {
		var port int
		retId, err := strconv.ParseInt(portStr, 10, 32)
		if err != nil {
			logx.Panic("port is error.")
		} else {
			port = int(retId)
		}
		maxChannelSize := 0
		if len(maxChannelSizeStr) > 0 {
			retId, err := strconv.ParseInt(maxChannelSizeStr, 10, 32)
			if err != nil {
				logx.Panic("maxChannelSize is error.")
			} else {
				maxChannelSize = int(retId)
			}
		}
		sc := serverItemConf{
			AddrConf:       *channel.NewAddrConf(serverIp, port),
			network:        network,
			maxChannelSize: maxChannelSize,
		}
		return append(serverItemConfs, sc)
	}
	return serverItemConfs
}

type serverItemConf struct {
	channel.AddrConf
	network        string
	maxChannelSize int
}

func initServerWsConf(config map[string]string) (string, []socket.IServerChildConf) {
	scheme := config[serverWsSchemeKey]
	index := 0
	wsConfs := make([]socket.IServerChildConf, 0)
	for {
		pathKey := fmt.Sprintf(serverWsKey+".%v.path", index)
		subproKey := fmt.Sprintf(serverWsKey+".%v.subprotocol", index)
		path := config[pathKey]
		if len(path) <= 0 {
			break
		}
		subpro := config[subproKey]
		logx.Infof("%v:%v", pathKey, path)
		logx.Infof("%v:%v", subproKey, subpro)
		wsConf := socket.NewServerChildConf(channel.NETWORK_WS, path)
		wsConf.AddAttach(socket.WS_SUBPROTOCOL_KEY, subpro)
		wsConfs = append(wsConfs, wsConf)
		index++
	}
	logx.Info("wsconfs:", wsConfs)
	// 如果为空则取默认
	if len(wsConfs) <= 0 {
		pathKey := serverKey + ".path"
		subproKey := serverKey + ".subprotocol"
		path := config[pathKey]
		subpro := config[subproKey]
		logx.Infof("%v:%v", pathKey, path)
		logx.Infof("%v:%v", subproKey, subpro)
		wsConf := socket.NewServerChildConf(channel.NETWORK_WS, path)
		wsConf.AddAttach(socket.WS_SUBPROTOCOL_KEY, subpro)
		wsConfs = append(wsConfs, wsConf)
	}
	return scheme, wsConfs
}

var serverLocationKey = "agent.server.location"

func initLocations(config map[string]string) []agent.ILocationConf {
	locationMap := make(map[string]string)
	for key, v := range config {
		if strings.Contains(key, serverLocationKey) {
			delete(config, key)
			locationMap[key] = v
		}
	}

	var locationConfs []agent.ILocationConf
	lcSize := len(locationMap)
	if lcSize > 0 {
		index := 0
		for {
			upstreamIdKey := fmt.Sprintf("agent.server.location.%v.upstreamId", index)
			upstreamId := locationMap[upstreamIdKey]
			if len(upstreamId) <= 0 {
				break
			}

			patternKey := fmt.Sprintf("agent.server.location.%v.pattern", index)
			pattern := locationMap[patternKey]
			if len(pattern) <= 0 {
				pattern = ""
			}
			locationConf := agent.NewLocationConf(pattern, upstreamId, nil)
			logx.Info("locationConf:", locationConf)
			if locationConfs == nil {
				locationConfs = make([]agent.ILocationConf, 1)
				locationConfs[index] = locationConf
			} else {
				locationConfs = append(locationConfs, locationConf)
			}
			index++
		}
	}
	return locationConfs
}

var key_prefix_agent = "agent."
var key_ch_readTimeout = key_prefix_agent + "channel.readTimeout"
var key_ch_writeTimeout = key_prefix_agent + "channel.writeTimeout"
var key_ch_readBufSize = key_prefix_agent + "channel.readBufSize"
var key_ch_writeBufSize = key_prefix_agent + "channel.writeBufSize"
var key_ch_closeRevFailTime = key_prefix_agent + "channel.closeRevFailTime"

func initChannelConf(config map[string]string) *channel.ChannelConf {
	defChConf := channel.NewDefChannelConf(channel.NETWORK_UNKNOWN)
	readBufSize := config[key_ch_readBufSize]
	if len(readBufSize) > 0 {
		retInt, err := strconv.ParseInt(readBufSize, 10, 32)
		if err == nil {
			defChConf.ReadBufSize = int(retInt)
		} else {
			logx.Info("error:", err)
		}
	}

	writeBufSize := config[key_ch_writeBufSize]
	if len(writeBufSize) > 0 {
		retInt, err := strconv.ParseInt(writeBufSize, 10, 32)
		if err == nil {
			defChConf.WriteBufSize = int(retInt)
		} else {
			logx.Info("error:", err)
		}
	}
	readTimeout := config[key_ch_readTimeout]
	if len(readTimeout) > 0 {
		retInt, err := strconv.ParseInt(readTimeout, 10, 32)
		if err == nil {
			defChConf.ReadTimeout = time.Duration(retInt)
		} else {
			logx.Info("error:", err)
		}
	}

	writeTimeout := config[key_ch_writeTimeout]
	if len(writeTimeout) > 0 {
		retInt, err := strconv.ParseInt(writeTimeout, 10, 32)
		if err == nil {
			defChConf.WriteTimeout = time.Duration(retInt)
		} else {
			logx.Info("error:", err)
		}
	}
	closeRevFailTime := config[key_ch_closeRevFailTime]
	if len(closeRevFailTime) > 0 {
		retInt, err := strconv.ParseInt(closeRevFailTime, 10, 32)
		if err == nil {
			defChConf.CloseRevFailTime = int(retInt)
		} else {
			logx.Info("error:", err)
		}
	}
	return defChConf
}
