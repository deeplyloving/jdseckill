package main

import (
	"fmt"
	"github.com/astaxie/beego/logs"
	"jdseckill/utils"
	"time"
)

var (
	quit              = make(chan int)
	killSuccess       = false
	orderStatus error = nil
)

func CmJdMaotaiProcessor(cookiesId string, fastModel bool) error {
	//TODO 初始化JdUtils
	logs.Info("UserId:", cookiesId)
	logs.Info("IsFast:", fastModel)
	jd := utils.NewJdUtils(cookiesId)
	jdTm := jd.GetJdTime()
	logs.Info("京东时间：", jdTm.Format(utils.TimeFormat))
	logs.Info("本地时间：", time.Now().Format(utils.TimeFormat))
	//TODO 验证是否登录，未登录扫码登录
	if err := jd.LoginByQCode(); err != nil {
		logs.Error(err.Error())
		return err
	}
	//TODO 删除图片
	utils.DeleteFile(jd.QrFilePath)
	//TODO 保存Cookies
	jd.Release()
	//TODO 获取用户名称
	if err := jd.GetUserName(); err != nil {
		return nil
	}

	//TODO 获取商品名称
	if err := jd.GetSkuTitle(); err != nil {
		logs.Error(err.Error())
		return err
	}

	//TODO 获取商品价格
	if err := jd.GetPrice(); err != nil {
		logs.Error(err.Error())
		return err
	}

	//TODO 预约商品
	jd.CommodityAppointment()

	weChatMessage := fmt.Sprintf(utils.MessageFormat, jd.UserName, jd.BuyTime, jd.SkuName, jd.SkuPrice, "成功", "未开始", "")
	if utils.AppConfig.MessageEnable {
		go jd.WeChatSendMessage(weChatMessage)
	}

	//TODO 定时任务，到达指定时间返回
	if err := jd.TaskCorn(); err != nil {
		return err
	}

	weChatMessage = fmt.Sprintf(utils.MessageFormat, jd.UserName, jd.BuyTime, jd.SkuName, jd.SkuPrice, "成功", "开始抢购", "")
	if utils.AppConfig.MessageEnable {
		go jd.WeChatSendMessage(weChatMessage)
	}

	count := 10
	for i := 0; i < count; i++ {
		go multiThreadingSkill(jd, fastModel)
	}

	for i := 0; i < count; i++ {
		<-quit
	}
	return orderStatus
}

func multiThreadingSkill(jd *utils.JdUtils, fastModel bool) error {
	defer func() {
		quit <- 1
	}()

	for {
		if killSuccess {
			orderStatus = nil
			return nil
		}
		//TODO 访问商品的抢购链接
		if err := jd.RequestSeckill(); err != nil {
			logs.Error(err.Error())
			continue
		}

		if !fastModel {
			//TODO 访问抢购订单结算页面
			if err := jd.RequestCheckOut(); err != nil {
				logs.Error(err.Error())
				continue
			}
		}
		//TODO 开始提交订单
		if err := jd.SubmitOrder(); err == nil {
			killSuccess = true
			orderStatus = nil
			return nil
		}
		nowTime := time.Now()
		if nowTime.Sub(jd.BuyTime).Seconds() > utils.AppConfig.StopSeconds {
			logs.Info("抢购时间以过【%f】分钟，自动停止...", utils.AppConfig.StopSeconds)
			weChatMessage := fmt.Sprintf(utils.MessageFormat, jd.UserName, jd.BuyTime, jd.SkuName, jd.SkuPrice, "成功", "抢购失败", "")
			if utils.AppConfig.MessageEnable {
				go jd.WeChatSendMessage(weChatMessage)
			}
			orderStatus = fmt.Errorf("抢购时间以过【%f】分钟，自动停止...", utils.AppConfig.StopSeconds)
			return orderStatus
		}
	}
}
