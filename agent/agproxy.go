/*
 * Author:slive
 * DATE:2020/8/16
 */
package agent

import (
	"errors"
	"gsfly/bootstrap"
	"gsfly/channel"
	gkcp "gsfly/channel/udpx/kcpx"
	logx "gsfly/logger"
)

type IProxy interface {
	IUpstream
}

type Proxy struct {
	Upstream

	LoadBalance LoadBalance
}

func NewProxy(parent interface{}, proxyConf IProxyConf) *Proxy {
	p := &Proxy{}
	p.Upstream = *NewUpstream(parent, proxyConf)
	return p
}

func (proxy *Proxy) SelectDstChannel(ctx *UpstreamContext) {
	lbsCtx := NewLbContext(nil, proxy, ctx.Channel)
	lbhandle := localBalanceHandles[proxy.LoadBalance.GetType()]
	lbhandle(lbsCtx)
	clientConf := lbsCtx.DstClientConf

	// 3、代理到目标
	var dstCh channel.IChannel
	params := ctx.Params[0].(map[string]interface{})
	clientPro := clientConf.GetProtocol()
	switch clientPro {
	case channel.PROTOCOL_WS, channel.PROTOCOL_HTTPX:
		handle := channel.NewDefChHandle(proxy.onDstChannelMsgHandle)
		handle.OnStopHandle = proxy.OnDstChannelStopHandle
		wsClientConf := clientConf.(*bootstrap.WsClientConf)
		wsClientConf.Params = params

		logx.Info("params:", wsClientConf.Params)
		wsClient := bootstrap.NewWsClient(proxy, wsClientConf, handle)
		err := wsClient.Start()
		agentCh := ctx.Channel
		if err != nil {
			logx.Error("dialws error, agentChId:" + agentCh.GetId())
			return
		}
		// 拨号成功，记录
		dstCh = wsClient.GetChannel()
		proxy.GetAgentChannelMap().Put(dstCh.GetId(), agentCh)
		proxy.GetDstChannelMap().Put(agentCh.GetId(), dstCh)
		break
	case channel.PROTOCOL_HTTP:
		break
	case channel.PROTOCOL_KWS00:
		break
	case channel.PROTOCOL_KWS01:
		break
	case channel.PROTOCOL_TCP:
		break
	case channel.PROTOCOL_UDP:
		break
	case channel.PROTOCOL_KCP:
		break
	default:
		// channel.PROTOCOL_WS
	}
	found := (dstCh != nil)
	if found {
		proxy.GetDstChannelPool().Put(dstCh.GetId(), dstCh)
	}
	ctx.SetRet(dstCh)
}

// onDstChannelMsgHandle upstream的客户端channel收到消息后的处理，直接会写到server对应的客户端channel
func (proxy *Proxy) onDstChannelMsgHandle(packet channel.IPacket) error {
	upsCtx := NewUpstreamContext(packet.GetChannel(), packet)
	proxy.QueryAgentChannel(upsCtx)
	agentChannel, found := upsCtx.GetRet(), upsCtx.IsOk()
	if found {
		// 回写，区分第一次，最后一次等？
		opcode := gkcp.OPCODE_TEXT_SIGNALLING
		oc := agentChannel.GetAttach(Opcode_Key)
		if oc != nil {
			cpc, ok := oc.(uint16)
			if ok {
				opcode = cpc
			}
		}
		frame := gkcp.NewOutputFrame(opcode, packet.GetData())
		srcPacket := agentChannel.NewPacket()
		srcPacket.SetData(frame.GetKcpData())
		agentChannel.Write(srcPacket)
		return nil
	} else {
		s := "src channel is nil, dst channel id:" + packet.GetChannel().GetId()
		logx.Error(s)
		return errors.New(s)
	}
}

// OnDstChannelStopHandle 当dstchannel关闭时，触发agentchannel关闭
func (proxy *Proxy) OnDstChannelStopHandle(dstChannel channel.IChannel) error {
	dstChId := dstChannel.GetId()
	defer func() {
		ret := recover()
		logx.Infof("finish to OnDstChannelStopHandle, chId:%v, ret:%v", dstChId, ret)
	}()

	// 当clientchannel关闭时，触发serverchannel关闭
	logx.Info("start to OnDstChannelStopHandle, chId:", dstChId)
	upstream := dstChannel.GetAttach("upstream")
	if upstream != nil {
		proxy, ok := upstream.(IProxy)
		if ok {
			agentCh, found := proxy.GetAgentChannelMap().Get(dstChId)
			if found {
				agetnChannel, ok := agentCh.(channel.Channel)
				if ok {
					agetnChannel.Stop()
					proxy.GetDstChannelMap().Remove(agetnChannel.GetId())
				}
				proxy.GetAgentChannelMap().Remove(agetnChannel.GetId())
				proxy.GetDstChannelPool().Remove(dstChId)
			}
		}
	}

	return nil
}

func (proxy *Proxy) TakeChannnelKey(ctx *UpstreamContext) (routeId string) {
	return ctx.Channel.GetId()
}

func (proxy *Proxy) QueryDstChannel(ctx *UpstreamContext) {
	InnerQueryDstChannel(proxy, ctx)
}

func (proxy *Proxy) QueryAgentChannel(ctx *UpstreamContext) {
	InnerQueryAgentChannel(proxy, ctx)
}