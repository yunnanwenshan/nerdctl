# nerdctl 使用指南

## 🚀 快速开始

### 1. 启动 containerd 环境
```bash
# 运行启动脚本
source /home/runner/app/start-containerd.sh
```

### 2. 使用 nerdctl 命令

#### 方法 A: 使用别名 (推荐)
```bash
nerdctl version
nerdctl images  
nerdctl ps
nerdctl pull alpine
```

#### 方法 B: 使用包装脚本
```bash
./nerdctl-wrapper.sh version
./nerdctl-wrapper.sh images
./nerdctl-wrapper.sh ps
```

#### 方法 C: 直接使用 (需要每次指定参数)
```bash
sudo /home/runner/app/_output/nerdctl --address /run/containerd/containerd.sock version
```

## 📋 常用命令示例

### 镜像管理
```bash
# 拉取镜像
nerdctl pull nginx:alpine

# 查看镜像
nerdctl images

# 删除镜像
nerdctl rmi nginx:alpine
```

### 容器管理
```bash
# 运行容器 (注意: 在当前环境中可能有限制)
nerdctl run --name test-container alpine echo "Hello nerdctl"

# 查看容器
nerdctl ps -a

# 删除容器
nerdctl rm test-container
```

### 网络和存储
```bash
# 查看网络
nerdctl network ls

# 查看卷
nerdctl volume ls

# 系统信息
nerdctl info
```

## 🔧 故障排除

### 如果看到 "rootless containerd not running" 错误:
1. 确保已运行启动脚本: `source /home/runner/app/start-containerd.sh`
2. 检查 containerd 是否运行: `ps aux | grep containerd`
3. 如果没有运行，手动启动: `sudo containerd &`

### 如果容器无法启动:
- 这是预期的，因为 Clacky 环境中有容器化限制
- 但是镜像管理、网络配置等功能都能正常工作

## 📚 学习和开发

现在您可以使用这个环境来:
- 📖 研究 nerdctl 的架构和实现
- 🔍 分析 Docker 兼容性机制  
- 🌐 理解 containerd 集成方式
- 🛠 测试容器管理工具的功能
- 💡 开发基于 containerd 的应用

## 🎯 环境状态

✅ **已配置完成:**
- containerd 运行时
- CNI 网络插件
- nerdctl 二进制文件
- 便捷的包装脚本和别名

⚠️ **已知限制:**
- 容器运行受到环境权限限制
- rootless 模式在当前环境中无法完全工作
- 某些高级功能需要额外的系统权限

---
🎉 **恭喜！您的 nerdctl 开发环境已就绪！**