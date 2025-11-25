# Flow Detection

基于流量分析的网络检测系统

## 📖 项目简介

Flow Detection 是一个用于网络流量检测和分析的系统，提供实时流量监控、异常检测和流量分析功能。

## 🚀 快速开始

### 环境要求

- Python 3.8+
- 其他依赖请查看 `requirements.txt`

### 安装

```bash
# 克隆项目
git clone <repository-url>
cd flow-detection

# 安装依赖
pip install -r requirements.txt
```

### 运行

```bash
# 启动应用
python main.py
```

## 📋 功能特性

- 🔍 实时流量监控
- 📊 流量统计分析
- ⚠️ 异常流量检测
- 📈 可视化展示
- 💾 数据持久化

## 🗂️ 项目结构

```
flow-detection/
├── docs/                    # 项目文档
│   └── git-workflow-guide.md  # Git 工作流规范
├── src/                     # 源代码目录
├── tests/                   # 测试代码
├── data/                    # 数据目录
├── config/                  # 配置文件
├── .github/                 # GitHub 配置
│   └── pull_request_template.md
├── .gitignore              # Git 忽略文件配置
├── LICENSE                 # 许可证
├── README.md              # 项目说明
└── requirements.txt       # Python 依赖

```

## 📚 文档

- [Git 工作流规范](docs/git-workflow-guide.md) - 项目的 Git 协作规范和最佳实践

## 🤝 贡献指南

欢迎贡献！在提交代码前，请务必阅读我们的 [Git 工作流规范](docs/git-workflow-guide.md)。

### 贡献流程

1. Fork 本项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'feat: 添加某个功能'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 创建 Pull Request

## 📝 提交规范

我们使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范。提交信息格式：

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Type 类型**：
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `style`: 代码格式调整
- `refactor`: 代码重构
- `perf`: 性能优化
- `test`: 测试相关
- `chore`: 构建/工具链修改

**示例**：
```
feat(detector): 添加 TCP 流量检测功能

实现了基于 TCP 协议的流量检测算法，支持连接状态跟踪和异常流量识别。

Closes #123
```

## 🔖 版本管理

本项目采用[语义化版本](https://semver.org/lang/zh-CN/)规范：`vMAJOR.MINOR.PATCH`

- **MAJOR**: 重大更新，不兼容的 API 变更
- **MINOR**: 向下兼容的功能新增
- **PATCH**: 向下兼容的问题修正

## 📄 许可证

本项目采用 [LICENSE](LICENSE) 许可证。

## 👥 团队

<!-- 添加团队成员信息 -->

## 📮 联系方式

<!-- 添加联系方式 -->

## 🙏 致谢

感谢所有为本项目做出贡献的开发者！

---

**最后更新时间**: 2025-11-24

