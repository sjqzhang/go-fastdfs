var xhrOnProgress=function(fun) {
    xhrOnProgress.onprogress = fun; //绑定监听
    //使用闭包实现监听绑
    return function() {
        //通过$.ajaxSettings.xhr();获得XMLHttpRequest对象
        var xhr = $.ajaxSettings.xhr();
        //判断监听函数是否为函数
        if (typeof xhrOnProgress.onprogress !== 'function')
            return xhr;
        //如果有监听函数并且xhr对象支持绑定时就把监听函数绑定上去
        if (xhrOnProgress.onprogress && xhr.upload) {
            xhr.upload.onprogress = xhrOnProgress.onprogress;
        }
        return xhr;
    }
}
var count = 0;
layui.use(['upload','element'], function() {
    var $ = layui.jquery, upload = layui.upload, element = layui.element;
    //多文件上传
    var demoListView = $('#moreFileList'),uploadListIns = upload.render({
        elem: '#fileList',
        url: '/file/upload/moreFileUpload',
        accept: 'file',
        multiple: true,
        auto: false,
        bindAction: '#fileListAction',
        xhr:xhrOnProgress,
        progress:function(value,obj){
            //上传进度回调 value进度值
            $("#moreFileList").find('.layui-progress ').each(function () {
                if(	$(this).attr("file")==obj.name){
                    var progressBarName=$(this).attr("lay-filter");
                    var percent = ((value.loaded / value.total)*100).toFixed(2);
                    if(percent == 100.00){
                        percent = 100;
                    }
                    element.progress(progressBarName, percent+'%');
                }
            })

        },
        choose: function(obj){
            //将每次选择的文件追加到文件队列
            var files = this.files = obj.pushFile();
            //读取本地文件
            obj.preview(function(index, file, result){
                count++;
                var tr = $(['<tr id="upload-'+ index +'">',
                    '<td>'+ file.name +'</td>',
                    '<td>'+ (file.size/1014).toFixed(1) +'kb</td>',
                    '<td>等待上传</td>',
                    '<td>' +
                    '<div file="'+file.name+'" class="layui-progress" lay-showPercent="true" lay-filter="progressBar'+count+'">'+
                    '<div class="layui-progress-bar layui-bg-green" lay-percent="0%"></div>'+
                    '</div>'+
                    '</td>',
                    '<td>',
                    '<button class="layui-btn layui-btn-xs more-file-reload layui-hide">重传</button>',
                    '<button class="layui-btn layui-btn-xs layui-btn-danger more-file-delete">删除</button>',
                    '</td>',
                    '</tr>'].join(''));
                //单个重传
                tr.find('.more-file-reload').on('click', function(){
                    obj.upload(index, file);
                });
                //删除
                tr.find('.more-file-delete').on('click', function(){
                    delete files[index];
                    tr.remove();
                    uploadListIns.config.elem.next()[0].value = '';
                });
                demoListView.append(tr);
                element.render('progress');
            });
        },before: function(obj){
            var scene = $("#scene").val();
            var path = $("#path").val();
            var showUrl = $("#showUrl").val();
            this.data={'scene': scene,'path':path,'showUrl':showUrl};
        },done: function(res, index, upload){
            //上传成功
            if(res.state == 200){
                var tr = demoListView.find('tr#upload-'+ index),tds = tr.children();
                tds.eq(2).html('<span style="color: #5FB878;">上传成功</span>');
                tds.eq(3).children(".layui-progress").children(".layui-progress-bar").attr("lay-percent","100%");
                //清空操作
                tds.eq(4).html('');
                tds.eq(4).html('<a class="layui-btn layui-btn-xs" target="_blank" onclick="showUrl(\''+res.data.url+'\');">查看链接</a>');
                //删除文件队列已经上传成功的文件
                return delete this.files[index];
            }else{
                var tr = demoListView.find('tr#upload-'+ index),tds = tr.children();
                tds.eq(2).html('<span style="color: #FF5722;">上传失败</span>');
                //显示重传
                tds.eq(4).find('.more-file-reload').removeClass('layui-hide');
            }
            this.error(index, upload);
        },error: function(index, upload){
            var tr = demoListView.find('tr#upload-'+ index),tds = tr.children();
            tds.eq(2).html('<span style="color: #FF5722;">上传失败</span>');
            //显示重传
            tds.eq(4).find('.more-file-reload').removeClass('layui-hide');
        }
    });
})

function showUrl(url) {
    layer.confirm('点击访问: <br><a href="'+url+'" target="_blank" title="点击访问" class="showUrl-href">'+url+'</a>', {
        btn: ['确定'],title:'查看链接'
    }, function(index, layero){
        layer.close(index);
    });
}