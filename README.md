# goautobuild

## 简介
监控文件变化，自动编译并运行go程序

## TODO
    1.自定义参数运行

## 参数
```
-d string
        监听的目录，默认当前目录.eg:/project (default "./")
  -e string
        监听的文件类型，默认监听所有文件类型.eg：'.go','.html','.php'
  -help
        显示帮助信息
  -i string
        忽略监听的目录
  -novendor
        编译是是否忽略vendor目录
```
## 安装
    go get -u -v github.com/iwannay/goautobuild

## 运行
```sh
// 将 goautobuild 所在路径加入环境变量
goautobuild -d $HOME/goproject/src/jiacrontab/server -e .go

```
