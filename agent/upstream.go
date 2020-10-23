/*
 * 路由是用户自定义方式实现
 * Author:slive
 * DATE:2020/8/13
 */
package agent

import (
	"github.com/Slive/gsfly/channel"
	"github.com/Slive/gsfly/common"
	logx "github.com/Slive/gsfly/logger"
	"github.com/emirpasic/gods/maps/hashmap"
)

// IUpstream upstream接口
type IUpstream interface {
	common.IParent

	GetConf() IUpstreamConf

	// 目标channel列表
	GetDstChannels() *hashmap.Map

	// channel对列表
	GetChannelPeers() *hashmap.Map

	// 获取channel对
	GetChannelPeer(ctx channel.IChHandleContext, isAgent bool) IChannelPeer

	// 初始化channel对
	InitChannelPeer(ctx channel.IChHandleContext, params ...interface{})

	// QueryDstChannel 查询dstChannel
	QueryDstChannel(ctx channel.IChHandleContext)

	// QueryAgentChannel 查询agentChannel
	QueryAgentChannel(ctx channel.IChHandleContext)

	ReleaseOnAgentChannel(agentCtx channel.IChHandleContext)

	ReleaseOnDstChannel(dstCtx channel.IChHandleContext)

	ReleaseChannelPeers()

	GetExtension() IExtension
}

type Upstream struct {
	common.Parent

	// 配置
	conf IUpstreamConf

	// 记录目标端channel池，可以复用
	dstChannels *hashmap.Map

	channelPeers *hashmap.Map

	extension IExtension
}

func NewUpstream(parent interface{}, upstreamConf IUpstreamConf, extension IExtension) *Upstream {
	if upstreamConf == nil {
		errMsg := "upstreamConf id is nil"
		logx.Error(errMsg)
		panic(errMsg)
	}

	u := &Upstream{
		conf:         upstreamConf,
		dstChannels:  hashmap.New(),
		channelPeers: hashmap.New(),
	}
	u.SetParent(parent)
	if extension == nil {
		u.extension = NewExtension()
	} else {
		u.extension = extension
	}
	return u
}

func (ups *Upstream) GetConf() IUpstreamConf {
	return ups.conf
}

// GetDstChannels 目标channel池, channelId作为主键
func (ups *Upstream) GetDstChannels() *hashmap.Map {
	return ups.dstChannels
}

// GetChannelPeers 获取channel对（agent/dst channel）
func (ups *Upstream) GetChannelPeers() *hashmap.Map {
	return ups.channelPeers
}

// GetChannelPeer 通过UpstreamContext获取到对应的channelpeer
func (ups *Upstream) GetChannelPeer(ctx channel.IChHandleContext, isAgent bool) IChannelPeer {
	panic("implement me")
}

func (ups *Upstream) GetExtension() IExtension {
	return ups.extension
}

func InnerQueryDstChannel(b IUpstream, ctx channel.IChHandleContext) {
	ret := b.GetChannelPeer(ctx, false)
	if ret != nil {
		peer := ret.(IChannelPeer)
		ctx.SetRet(peer.GetDstChannel())
	} else {
		logx.Warn("query dst is not existed.")
	}
}

func InnerQueryAgentChannel(b IUpstream, ctx channel.IChHandleContext) {
	ret := b.GetChannelPeer(ctx, true)
	if ret != nil {
		peer := ret.(IChannelPeer)
		ctx.SetRet(peer.GetAgentChannel())
	} else {
		logx.Warn("query agent is not existed.")
	}
}

func (ups *Upstream) ReleaseChannelPeers() {
	upsId := ups.GetConf().GetId()
	defer func() {
		ret := recover()
		if ret != nil {
			logx.Errorf("start to clear upstream, id:%v, ret:%v", upsId, ret)
		} else {
			logx.Info("finish to clear upstream, id:", upsId)
		}
	}()
	logx.Info("start to clear upstream, id:", upsId)
	ups.channelPeers.Clear()

	// 释放所有dstchannel
	dstChPool := ups.GetDstChannels()
	dstVals := dstChPool.Values()
	for _, val := range dstVals {
		val.(channel.IChannel).Stop()
	}
	dstChPool.Clear()
}

// IChannelPeer channel对（agent/dstchannel）
type IChannelPeer interface {
	GetAgentChannel() channel.IChannel

	GetDstChannel() channel.IChannel

	common.IAttact
}

// ChannelPeer channel对（agent/dstchannel)，通过路由后得到的channelpeer
type ChannelPeer struct {
	agentChannel channel.IChannel

	dstChannel channel.IChannel

	common.Attact
}

func NewChannelPeer(agentChannel, dstChannel channel.IChannel) *ChannelPeer {
	return &ChannelPeer{
		agentChannel: agentChannel,
		dstChannel:   dstChannel,
	}
}

func (cp *ChannelPeer) GetAgentChannel() channel.IChannel {
	return cp.agentChannel
}

func (cp *ChannelPeer) GetDstChannel() channel.IChannel {
	return cp.dstChannel
}