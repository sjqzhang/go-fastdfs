# 背景{#background}

## 现在开源的分布式文件系非常多，功能也很强大，那为什么还要重新造轮子呢？可能这是很多人都关心的问题。重复造轮主要解决以下几个问题：
- 解决软件架构复杂问题
- 解决运维部署维护问题
- 解决使用复杂到处找文档


# go-fastdfs是什么？ {#what}
### go-fastdfs是一个基于http协议的分布式文件系统，它基于大道至简的设计理念，一切从简设计，使得它的运维及扩展变得更加简单，它具有高性能、高可靠、无中心、免维护等优点。

#### 大家担心的是这么简单的文件系统，靠不靠谱，可不可以用于生产环境？答案是肯定的，正因为简单所以高效，因为简单所以稳定。如果你担心功能，那就跑单元测试，如果担心性能，那就跑压力测试，项目都自带了，跑一跑更放心^_^。

注意：使用前请认真阅读完本文，特别是[wiki](https://github.com/sjqzhang/go-fastdfs/wiki)

# 特性{#advantage}

- 支持curl命令上传
- 支持浏览器上传
- 支持HTTP下载
- 支持多机自动同步
- 支持断点下载
- 支持配置自动生成
- 支持小文件自动合并(减少inode占用)
- 支持秒传
- 支持跨域访问
- 支持一键迁移
- 支持并行体验
- 支持断点续传([tus](https://tus.io/))
- 支持docker部署
- 支持自监控告警
- 支持图片缩放
- 支持google认证码
- 支持自定义认证
- 支持集群文件信息查看
- 使用通用HTTP协议
- 无需专用客户端（支持wget,curl等工具）
- 类fastdfs
- 高性能 （使用leveldb作为kv库）
- 高可靠（设计极其简单，使用成熟组件）
- 无中心设计(所有节点都可以同时读写)

# 优点

- 无依赖(单一文件）
- 自动同步
- 失败自动修复
- 按天分目录方便维护
- 支持不同的场景
- 文件自动去重
- 支持目录自定义
- 支持保留原文件名
- 支持自动生成唯一文件名
- 支持浏览器上传
- 支持查看集群文件信息
- 支持集群监控邮件告警
- 支持小文件自动合并(减少inode占用)
- 支持秒传
- 支持图片缩放
- 支持google认证码
- 支持自定义认证
- 支持跨域访问
- 极低资源开销
- 支持断点续传([tus](https://tus.io/))
- 支持docker部署
- 支持一键迁移（从其他系统文件系统迁移过来）
- 支持并行体验（与现有的文件系统并行体验，确认OK再一键迁移）
- 支持token下载　token=md5(file_md5+timestamp)
- 运维简单，只有一个角色（不像fastdfs有三个角色Tracker Server,Storage Server,Client），配置自动生成
- 每个节点对等（简化运维）
- 所有节点都可以同时读写


[![asciicast](https://asciinema.org/a/258926.svg)](https://asciinema.org/a/258926)
# 启动服务器（已编译，[下载](https://github.com/sjqzhang/fastdfs/releases)极速体验，只需一分钟）
一键安装：（请将以下命令复制到linux console中执行）
```shell
wget --no-check-certificate  https://github.com/sjqzhang/go-fastdfs/releases/download/v1.3.1/fileserver -O fileserver && chmod +x fileserver && ./fileserver
```
(注意：下载时要注意链接的版本号，windows下直接运行fileserver.exe，执行文件在这里[下载](https://github.com/sjqzhang/fastdfs/releases))



部署图
![部署图](doc/go-fastdfs-deploy.png)

通用文件认证时序图
![通用文件认证时序图](doc/authentication2.png)

文件google认证时序图
![文件认证时序图](doc/authentication.png)

# 有问题请[点击反馈](https://github.com/sjqzhang/go-fastdfs/issues/new)

# 重要说明
## 在issue中有很多实际使用的问题及回答（很多已关闭，请查看已关闭的issue）

## 项目从v1.1.8开始进入稳定状态

# 更新说明
## 从低版升给到高版本，可能存在配置项变动的情况，一定要注意使用新的版本时的配置项。如何获得新版本的配置项及说明？先备份旧的配置项（文件名不能为cfg.json），再运行新的版本，配置项就自动生成。然后再修改相应的配置项。

- v1.1.9 增加文件自动迁移功能，支持同名文件重复覆盖选项。

 # <span id="qa">Q&A</span>

