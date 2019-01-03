# 简单分布式文件服务器（类fastdfs）在运维管理等方面优于fastdfs

- 支持curl命令上传
- 支持浏览器上传
- 支持HTTP下载
- 支持多机自动同步
- 类fastdfs
- 高性能
- 高可靠

# 优点

- 无依赖(单一文件）
- 自动同步
- 失败自动修复
- 按天分目录方便维护
- 文件自动去重
- 支持浏览器上传
- 运维简单，只有一个角色，配置自动生成
- 每台机器对等



# 启动服务器（已编译，[下载直接](https://github.com/sjqzhang/FileServer/releases)）

`./fileserver`

# 配置  (conf/cfg.json)
```json
{
  "绑定端号":"端口",
  "addr": ":8080",
  "集群":"集群列表",
  "peers":["http://10.1.50.xx:8080","http://10.1.14.xx:8080","http://10.1.50.xx:8080"],
  "组号":"组号",
  "group":"group1",
  "refresh_interval":120,
  "是否自动重命名":"真假",
  "rename_file":false,
  "是否支持ＷＥＢ上专":"真假",
  "enable_web_upload":true,
  "是否显示目录":"真假",
  "show_dir":true
}
```


# 上传命令

`curl -F file=@http-index-fs http://10.1.50.90:8080/upload` 	


# WEB上传（浏览器打开）

`http://127.0.0.1:8080` 	
