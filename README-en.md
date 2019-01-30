# [中文](README.md)  [English](README-en.md)
![logo](doc/logo.png)
# Distributed file server go-fastdfs (similar fastdfs) is better than fastdfs in terms of operation and maintenance management, more humane

- Support curl command upload
- Support browser upload
- Support HTTP download
- Support multi-machine automatic synchronization
- similar fastdfs
- High performance (using leveldb as a kv library)
- High reliability (design is extremely simple, using mature components)

# advantage

- No dependencies (single file)
- Automatic synchronization
- Failure automatic repair
- Convenient maintenance by talent directory
- Support different scenarios
- Automatic file deduplication
- Support for directory customization
- Support to retain the original file name
- Support for automatic generation of unique file names
- Support browser upload
- Support for viewing cluster file information
- Support cluster monitoring email alarm
- Support token download token=md5(file_md5+timestamp)
- Easy operation and maintenance, only one role (unlike fastdfs has three roles Tracker Server, Storage Server, Client), the configuration is automatically generated
- Peer-to-peer (simplified operation and maintenance)
- All nodes can read and write simultaneously



#Start the server (compiled, [download](https://github.com/sjqzhang/fastdfs/releases) experience)
```
./fileserver
```


# Configure automatic generation (conf/cfg.json)
```json
{
	"addr": ":8080",
	"peers": ["http://10.1.xx.2:8080", "http://10.1.xx.5:8080", "http://10.1.xx.60:8080"],
	"group": "group1",
	"refresh_interval": 120,
	"rename_file": false,
	"enable_web_upload": true,
	"enable_custom_path": true,
	"download_domain": "",
	"scenes": [],
	"default_scene": "default",
	"show_dir": true,
	"mail": {
		"user": "abc@163.com",
		"password": "abc",
		"host": "smtp.163.com:25"
	},
	"alram_receivers": [],
	"alarm_url": "",
	"download_use_token": false,
	"download_token_expire": 600,
	"auto_repair": true
}
```


#Command upload

`curl -F file=@http-index-fs http://10.1.xx.60:8080/upload`


# WEB upload (browser open)

`http://127.0.0.1:8080`

#Code upload (options see browser upload)

```python
import requests
url = 'http://127.0.0.1:8080/upload'
files = {'file': open('report.xls', 'rb')}
options={'output':'json','path':'','scene':''} #See browser upload options
r = requests.post(url, files=files)
print(r.text)
```


# Please click [Feedback](https://github.com/sjqzhang/go-fastdfs/issues/new)


# Q&A
- Can files already stored using fastdfs be migrated to go fastdfs?
```
The answer is yes, the problem you are worried about is the path change, go fastdfs considers this for you.

curl -F file=@data/00/00/_78HAFwyvN2AK6ChAAHg8gw80FQ213.jpg -F path=M00/00/00/ http://127.0.0.1:8080/upload

Similarly, all files can be migrated with one line of command.

cd fastdfs/data && find -type f |xargs -n 1 -I {} curl -F file=@data/{} -F path=M00/00/00/ http://127.0.0.1:8080/

The above commands can be moved rough
Can write some simple scripts for migration

```

- Need to install nginx yet?
```
Can not be installed, you can also choose to install
Go fastdfs itself is a high performance web file server.
```

- How to view cluster file information?
```
Http://10.1.xx.60:8080/stat

What should I do if there is a file error?
Please delete the stat.json file in the data directory. Restart the service, please recalculate the number of files automatically.
```
- How reliable can it be used in a production environment?
```
This project has been used on a large scale in the production environment, such as fear of not meeting
You can stress test its features before use, any
The problem can be directly mentioned
```

- Can I have multiple servers on one machine?
```
No, the high availability of the cluster has been considered at the beginning of the design. In order to ensure the true availability of the cluster, it must be a different ip.
Error "peers": ["http://127.0.0.1:8080","http://127.0.0.1:8081","http://127.0.0.1:8082"]
Correct "peers": ["http://10.0.0.3:8080","http://10.0.0.4:8080","http://10.0.0.5:8082"]
```
- What should I do if the files are not synchronized?
```
Under normal circumstances, the cluster automatically synchronizes the repair files every hour. (The performance is poor, it is recommended to turn off automatic repair in case of massive)
What about the abnormal situation?
Answer: Manual synchronization
Http://172.16.70.123:7080/sync?date=20190117&force=1
Parameter description: date indicates the data of the day of synchronization. force 1. indicates whether to force synchronization of all the day (poor performance), 0. means that only failed files are synchronized.

Unsynchronized situation:
1) Originally running N sets, now suddenly join one to become N+1
2) Originally running N sets, one machine has a problem and becomes N-1

```

- Does the file out of sync affect access?
```
Answer: It will not affect, it will automatically repair the files that are not synchronized when the access is not available.
```


- How to test?
```
First use gen_file.py to generate a large number of files (note that if you want to generate large files, you can multiply the content by a large number)
E.g:
# -*- coding: utf-8 -*-
import os
j=0
for i in range(0,1000000):
    if i%1000==0:
        j=i
        os.system('mkdir %s'%(i))
    with open('%s/%s.txt'%(j,i),'w+') as f:
        f.write(str(i)*1024)
Then use benchmark.py for pressure measurement
It is also possible to perform pressure measurement simultaneously in multiple machines, and all nodes can be read and written simultaneously.
```


- If you have any questions, please click [Reply](https://github.com/sjqzhang/go-fastdfs/issues/new)

![wechat](doc/wechat.jpg)
