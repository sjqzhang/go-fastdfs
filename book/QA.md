# 最佳实战？
```
一、如果是海量存储，不要开启文件token认证功能，减少性能开消。
二、尽量用标准上传，上传后业务保存path，在业务用的时候再并接上域名（方便迁移扩展等）。
三、如果使用断点续传，上传后一定要用文件id置换成path存储（如何置换看ＱＡ/API文档），为后面访问减少性能开消。
四、尽量使用物理服务器部署，因为主要压力或性能来自于IO
五、线上业务尽量使用nginx+gofastdfs部署架构(均衡算法使用ip_hash)，以满足后面的功能扩展性(nginx+lua)。
六、线上环境最好不要使用容器部署，容器适用于测试和功能验证。
总结：业务保存的文件的path,减少后期访问路径转换带来开消,文件访问权限由业务来完成，这样性能最好，通用性强（可直接其它web服务器）。

重要提醒：如果开启小文件合并功能，后期是无法删除小文件的。
上传结果说明
请使用md5,path,scene字段，其它是为了兼容老的线上系统添加的，以后有可能去掉。

```


## 有管理后台么？

```
https://github.com/perfree/go-fastdfs-web
```
## 断点上传有使用说明么？

```
https://github.com/tus
```

## 在微信讨论群中大家都问到go-fastdfs性能怎样？

```
由于问的人太多，在这里统一回答。
go-fastdfs的文件定位与其它分布式系统不同，它的寻址是直接定位，不经过任何组件，所以可以近似时间复杂度为o(1)[文件路径定位]
基本没有性能损耗，项目中也附有压测脚本，大家可以自已进行压测，群里就不要太多讨论问题了，人多每次回复同样的问题
大家也会觉得这群无聊。
```



## 已经使用fastdfs存储的文件可以迁移到go-fastdfs下么(其它类似)？

```
答案是可以的，你担心的问题是路径改变,go fastdfs为你考虑了这一点
步骤：
	一、下载最新版的go-fastdfs
	二、将原来的fastdfs文件目录复制到go-fastdfs的 files目录下(如果文件很多，可以逆向过来，将fileserver复制过去，但要保留fileserver的目录结构）
	三、将配置enable_migrate设为true
	注意：迁移过程中会扫描整下files目录下的所有文件，
	速度较慢，迁移完成后请将enable_migrate设为false

说明：go-fastdfs的目录是不能变动的，与同步机制相关，很多同学在群里问题，我的files目录能不能自定义，答案是否定的。
至于能不能用软链接的方式本人没有测试过，可以自行测试。

```

## 什么是集群，如何用Nginx管理多集群？

```
1、在go-fastdfs中，一个集群就是一个group。
2、请参阅部署图
注意：配置中的 support_group_manage 参数设为true时，所有的url中都自动添加组信息。
例如：http://10.1.5.9:8080/group/status
默认：http://10.1.5.9:8080/status
区别：多了group,对应配置中的 group 参数,这样主要是为了解决一个Nginx反向代理多个group(集群)
具体请参阅部署图

```


## 如何搭建集群？

```
一、先下载已编译的可执行文件（用最新版本）
二、运行可执行文件（生成配置）
三、修改配置
	peers：增加对端的http地址
	检查:
	host:自动生成是否正确
	peer_id:集群内是否唯一
四、重新运行服器
五、验证服务是否OK
```


## 适合海量存储吗？

```
答案：适合海量存储
```

## 如何上传文件夹？

```
 DIR=log &&  ls $DIR |xargs -n 1 -I {} curl -s  -F path=$DIR  -F file=@$DIR/{} http://10.1.50.90:8080/upload 
 上面命令的log为要上传的目录，如果代码上传就是简单的循环上传就ＯＫ。
```

## 如何缩放图片？

```
在下载url中加入width各height参数
例如：http://127.0.0.1:8080/group1/haystack/5/124,0,27344,.jpg?download=0&width=100&height=100
特明说明是：如果要等比例缩放，请将width或height设为０
```

## 如何在浏览器中直接显示图片？

```
在下载url中加入download=0参数
例如：http://127.0.0.1:8080/group1/haystack/5/124,0,27344,.jpg?download=0
```


## 如何实现自定义认证上传下载？

```
一、使用1.2.6版本以后的go-fastdfs
二、设auth_url参数（应用提供）
三、应用实现验证权限接口（即第二步的url）,参数为　auth_toke 返回　ok 表示认证通过，其它为不通过
四、认证通过后，可以上传或下载
```


## 还需要安装nginx么？

```
go-fastdfs本身是一个高性能的web服务器，在开发或测试时，可以不用安装nginx，
但go-fastdfs的功能单一，如需要缓存或重定向或其它扩展，nginx都有成熟的组件
所以建议线上还是加一层nginx，再借助nginx+lua解决扩展性问题。
```

## 能动态加载配置么？

```
答案：是可以的，但要更新到最新版本
步骤：
1）修改 conf/cfg.json 文件
2）访问 http://10.1.xx.60:8080/reload
3） 注意：每个节点都需要进行同样的操作
```


## 如何查看集群文件信息？

```
http://10.1.xx.60:8080/stat

如果出现文件统计出错怎么办？
请删除 data目录下的 stat.json文件 重启服务，请系统自动重新计算文件数。

或者调用
http://10.1.xx.60:8080/repair_stat

```
## 可靠性怎样，能用于生产环境么？
```
本项目已大规模用于生产环境，如担心不能满足
可以在使用前对其各项特性进行压力测试，有任何
问题可以直接提issue
```

## 如何后台运行程序？

```
请使用control 对程序进行后面运行，具体操作如下:
    一、 chmod +x control
    二、 确保control与fileserver在同一个目录
    三、 ./control start|stop|status #对和序进行启动，停止，查看状态等。 

```


## 能不能在一台机器部置多个服务端？

```
不能，在设计之初就已考虑到集群的高可用问题，为了保证集群的真正可用，必须为不同的ip,ip 不能用 127.0.0.1
错误　"peers": ["http://127.0.0.1:8080","http://127.0.0.1:8081","http://127.0.0.1:8082"]
正确　"peers": ["http://10.0.0.3:8080","http://10.0.0.4:8080","http://10.0.0.5:8080"]
```
## 文件不同步了怎么办？

```
正常情况下，集群会每小时自动同步修复文件。（性能较差，在海量情况下建议关闭自动修复）
那异常情况下怎么？
答案：手动同步（最好在低峰执行）
http://172.16.70.123:7080/sync?date=20190117&force=1 (说明：要在文件多的服务器上执行，相关于推送到别的服务器)
参数说明：date 表示同步那一天的数据　force　1.表示是否强制同步当天所有(性能差)，0.表示只同步失败的文件

不同步的情况：
1) 原来运行N台，现在突然加入一台变成N+1台
2）原来运行N台，某一台机器出现问题，变成N-1台

如果出现多天数据不一致怎么办？能一次同步所有吗？
答案是可以：(最好在低峰执行)
http://172.16.70.123:7080/repair?force=1

```

## 文件不同步会影响访问吗？

```
答案：不会影响，会在访问不到时，自动修复不同步的文件。
```

## 如何查看系统状态及说明？

```
http://172.16.70.123:7080/status
注意:（Fs.Peers是不带本机的，如果带有可能出问题）
本机为 Fs.Local
sts["Fs.ErrorSetSize"] = this.errorset.Cardinality()  这个会导致内存增加

```


## 如何编译(go1.9.2+)？

```
git clone https://github.com/sjqzhang/go-fastdfs.git
cd go-fastdfs
mv vendor src
pwd=`pwd`
GOPATH=$pwd go build -o fileserver fileserver.go
```

## 如何跑单元测试 (尽量在linux下进行)？

```
git clone https://github.com/sjqzhang/go-fastdfs.git
cd go-fastdfs
mv vendor src
pwd=`pwd`
GOPATH=$pwd go test -v fileserver.go fileserver_test.go

```



## 如何压测？

```
步骤：
一、创建files文件夹
二、将gen_file.py复制到files文件夹中，通过python gen_file.py 生成大量文件
三、将benchmark.py放到 files目录外（即与files目录同一级），通过python benchmark.py进行压测（注意对benchmark.py中的ip进行修改）
先用gen_file.py产生大量文件（注意如果要生成大文件，自已在内容中乘上一个大的数即可）
例如:
# -*- coding: utf-8 -*-
import os
j=0
for i in range(0,1000000):
    if i%1000==0:
        j=i
        os.system('mkdir %s'%(i))
    with open('%s/%s.txt'%(j,i),'w+') as f:
        f.write(str(i)*1024)
接着用benchmark.py进行压测
也可以多机同时进行压测，所有节点都是可以同时读写的
```



## 支持断点下载？

```
答案：支持
curl wget 如何
wget -c http://10.1.5.9:8080/group1/default/20190128/16/10/2G
culr -C - http://10.1.5.9:8080/group1/default/20190128/16/10/2G
```

## Docker如何部署？

```
步骤：
方式一、
    一、构建镜像
    docker build . -t fastdfs
    二、运行容器（使用环境变量 GO_FASTDFS_DIR 指向存储目录。）
    docker run --name fastdfs -v /data/fastdfs_data:/data -e GO_FASTDFS_DIR=/data fastdfs 
方式二、
    一、拉取镜像
    docker pull sjqzhang/go-fastdfs
    二、运行容器
    docker run --name fastdfs -v /data/fastdfs_data:/data -e GO_FASTDFS_DIR=/data fastdfs 

```

## 大文件如何分块上传或断点续传？

```
一般的分块上传都要客户端支持，而语言的多样性，客户端难以维护，但分块上传的功能又有必要，为此提供一个简单的实现思路。
方案一、
借助linux split cat 实现分割与合并，具体查看 split 与　cat 帮助。
分割： split  -b 1M filename #按每个文1M
合并： cat  x* > filename #合并
方案二、
借助hjsplit
http://www.hjsplit.org/
具体自行实现
方案三、
建议用go实现hjsplit分割合并功，这样具有跨平台功能。（未实现，等你来....）
方案四、
使用内置的继点续传功能（使用protocol for resumable uploads协议，[详情](https://tus.io/)）
 注意：方案四、只能指定一个上传服务器，不支持同时写，并且上传的url有变化
 原上传url： http://10.1.5.9:8080/<group>/upload
 断点上传url： http://10.1.5.9:8080/<group>/big/upload/
 上传完成，再通过秒传接口，获取文件信息
```

## 如何秒传文件？

```
通过http get的方式访问上传接口
http://10.0.5.9:8080/upload?md5=filesum&output=json
参数说明：
md5=sum(file) 文件的摘要算法要与文件务器的算法一致（算法支持md5|sha1），如果是断点续传，可以使用文件的id，也就是urlolad后的id
output=json|text 返回的格式
 
```

## 集群如何规划及如何进行扩容？

```
建议在前期规划时，尽量采购大容量的机器作为存储服务器，如果要两个副本就用两台组成一个集群，如果要三个副本
就三台组成一个集群。（注意每台服务器最好配置保持一样，并且使用raid5磁盘阵列）

如果提高可用性，只要在现在的集群peers中加入新的机器，再对集群进行修复即可。
修复办法 http://172.16.70.123:7080/repair?force=1  （建议低峰变更）

如何扩容？
为简单可靠起见，直接搭建一个新集群即可（搭建就是启动./fileserver进程，设置一下peers的IP地址，三五分钟的事）
issue中chengyuansen同学向我提议使用增加扩容特性，我觉得对代码逻辑及运维都增加复杂度，暂时没有加入这特性。

```


# 访问限制问题

```
出于安全考虑,管理API只能在群集内部调用或者用127.0.0.1调用.
```




## 有问题请[点击反馈](https://github.com/sjqzhang/go-fastdfs/issues/new)
## 有问题请加群
![二维码](data:image/jpeg;base64,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)

#### 进群请改昵称，昵称格式：城市－公司－昵称，如果你喜欢这项目，请关注(star)此项目，关注是对项目的肯定，也是作者创新的动力。

#### [捐赠](https://raw.githubusercontent.com/sjqzhang/go-fastdfs/master/doc/pay.png)

