# 分布式文件服务器（类fastdfs）在运维管理等方面优于fastdfs，更人性化

- 支持curl命令上传
- 支持浏览器上传
- 支持HTTP下载
- 支持多机自动同步
- 类fastdfs
- 高性能 （使用leveldb作为kv库）
- 高可靠（设计极其简单，使用成熟组件）

# 优点

- 无依赖(单一文件）
- 自动同步
- 失败自动修复
- 按天分目录方便维护
- 文件自动去重
- 支持目录自定义
- 支持保留原文件名
- 支持自动生成唯一文件名
- 支持浏览器上传
- 运维简单，只有一个角色（不像fastdfs有三个角色Tracker Server,Storage Server,Client），配置自动生成
- 每个节点对等（简化运维）



# 启动服务器（已编译，[下载](https://github.com/sjqzhang/fastdfs/releases)体验）

`./fileserver`

# 配置自动生成  (conf/cfg.json)
```json
{
	"绑定端号": "端口",
	"addr": ":8080",
	"集群": "集群列表例如： http://10.1.xx.2:8080,http://10.1.xx.5:8080,http://10.1.xx.60:8080,注意是字符串数组",
	"peers": ["%s"],
	"组号": "组号",
	"group": "group1",
	"refresh_interval": 120,
	"是否自动重命名": "真假",
	"rename_file": false,
	"是否支持web上传": "真假",
	"enable_web_upload": true,
	"是否支持非日期路径": "真假",
	"enable_custom_path": true,
	"下载域名": "dowonload.web.com",
	"download_domain": "",
	"场景":"场景列表",
	"scenes":[],
	"默认场景":"",
	"default_scene":"default",
	"是否显示目录": "真假",
	"show_dir": true
}
```


# 命令上传

`curl -F file=@http-index-fs http://10.1.xx.60:8080/upload` 	


# WEB上传（浏览器打开）

`http://127.0.0.1:8080` 	

# Q&A
- 已经使用fastdfs存储的文件可以迁移到go fastdfs下么？
```
答案是可以的，你担心的问题是路径改变,go fastdfs为你考虑了这一点

curl -F file=@data/00/00/_78HAFwyvN2AK6ChAAHg8gw80FQ213.jpg -F path=M00/00/00/ http://10.1.50.90:8080/upload

同理可以用一行命令迁移所有文件

cd fastdfs/data && find -type f |xargs -n 1 -I {} curl -F file=@data/{} -F path=M00/00/00/ http://10.1.50.90:8080/

以上命令可以过迁粗糙
可以写一些简单脚本进行迁移

```

- 还需要安装nginx么？
```
可以不安装，也可以选择安装
go fastdfs 本身就是一个高性能的web文件服务器。
```

