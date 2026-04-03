# wechat-analyzer

微信聊天记录 AI 分析工具

基于 wechat-export 导出的 JSON 文件，做统计分析和 AI 人格画像。

## 功能特性

- 📊 **基础统计分析**：
  - 总消息数、平均消息长度、日均消息数
  - 发送/接收比例、谁先开口的比例
  - 活跃时段分布、各类型消息比例
- 🧠 **AI 人格画像**（基于大模型）：
  - 人格称号（如"深夜代码诗人"、"灵魂伙伴"）
  - 人格类型（类似 MBTI 的四字类型）
  - 人格标签（4 个关键词）
  - 人格画像、关系分析、常聊话题
- 📈 **数据可视化**：
  - ECharts 生成发送比例饼图、活跃时段柱状图
  - 自包含 HTML 报告，无需联网即可查看
- 🤖 **多 AI 提供商支持**：
  - DeepSeek、Kimi、通义千问、豆包、智谱 GLM、Claude
  - 自动检测已配置的提供商，无需手动指定

## 快速开始

### 前置准备

1. 用 `wechat-export` 导出聊天记录为 JSON 格式
2. 设置至少一个 AI 提供商的 API Key

```bash
# 以 DeepSeek 为例
export DEEPSEEK_API_KEY=your_key_here
```

### 安装

```bash
# 方法一：从 GitHub 安装
go install github.com/superShen0916/wechat-analyzer@latest

# 方法二：下载源码编译
git clone https://github.com/superShen0916/wechat-analyzer.git
cd wechat-analyzer
go build -o wechat-analyzer ./cmd
```

### 基本用法

#### 统计分析

```bash
# 单个文件分析
wechat-analyzer stats ./output/张三.json

# 生成 HTML 统计报告
wechat-analyzer stats ./output/张三.json --html

# 分析目录下所有聊天记录
wechat-analyzer stats ./output/
```

#### AI 人格分析

```bash
# 分析单个聊天记录
wechat-analyzer analyze ./output/张三.json

# 指定 AI 提供商
wechat-analyzer analyze ./output/张三.json --provider deepseek

# 生成 AI 分析 HTML 报告
wechat-analyzer analyze ./output/张三.json --html

# 分析并导出报告到指定目录
wechat-analyzer analyze ./output/张三.json --html --output ./reports
```

#### 列出支持的提供商

```bash
wechat-analyzer providers
```

## 支持的 AI 提供商

| 提供商 | 参数 | 环境变量 | 推荐指数 |
|--------|------|----------|----------|
| DeepSeek | `--provider deepseek` | `DEEPSEEK_API_KEY` | ⭐️⭐️⭐️⭐️⭐️ |
| Kimi | `--provider moonshot` | `MOONSHOT_API_KEY` | ⭐️⭐️⭐️⭐️ |
| 通义千问 | `--provider qwen` | `DASHSCOPE_API_KEY` | ⭐️⭐️⭐️⭐️ |
| 豆包 | `--provider doubao` | `DOUBAO_API_KEY` | ⭐️⭐️⭐️ |
| 智谱 GLM | `--provider zhipu` | `ZHIPU_API_KEY` | ⭐️⭐️⭐️ |
| Claude | `--provider anthropic` | `ANTHROPIC_API_KEY` | ⭐️⭐️⭐️⭐️ |

## 输出说明

### 统计分析输出示例

```
📊 聊天记录统计 (张三)
───────────────────────────
              总消息数:   12345 条
          平均每条长度:    12.3 字符
              日均消息数:    3.2 条/天

           我发的消息:    4567 (37.0%)
             对方发的:    7778 (63.0%)
           我先开口:    345 (2.8%)

⏰ 活跃时段分布:
  20点:21点 → 1234条 ■■■■■■■■■■  (峰值)
  21点:22点 → 1123条 ■■■■■■■■■
  19点:20点 → 987条  ■■■■■■■■
```

### AI 人格分析输出示例

```
=======================================================================
🎭 AI 分析结果
=======================================================================
人格称号：深夜灵魂伴侣
人格类型：ENFP-理想主义者
人格标签：#深夜修仙  #灵魂共鸣  #温暖陪伴  #爱说废话

人格画像：
  对方是一个温暖而富有同理心的人，喜欢在深夜聊天，善于倾听...

关系分析：
  你们的沟通非常平等，彼此都是对方的情感树洞，经常聊到深夜...

常聊话题：
  生活感悟、电影音乐、未来梦想

一句话总结：
  深夜里的灵魂共鸣，心灵上的完美契合
=======================================================================
```

## 技术细节

### 支持的导出格式

该工具仅支持解析 `wechat-export` 导出的 JSON 文件，JSON 格式定义：

```json
{
  "talker": {
    "user_name": "wxid_xxx",
    "nick_name": "张三",
    "is_group": false
  },
  "total": 12345,
  "exported_at": "2024-01-01T00:00:00+08:00",
  "messages": [
    {
      "local_id": 12345,
      "type": 1,
      "type_name": "text",
      "is_sender": true,
      "create_time": 1704067200,
      "content": "你好",
      "display_content": "你好"
    }
  ]
}
```

### AI Prompt 设计

使用结构化 Prompt 引导大模型输出：

```
你是一个顶尖的微信聊天记录分析专家，对人性洞察敏锐且幽默犀利。

请严格按照以下结构输出：
1. 人格称号（4-8 字有趣称号）
2. 人格类型（类似 MBTI 的四字类型）
3. 人格标签（4 个标签，用 # 号分隔）
4. 人格画像（生动描述对方性格，200 字内）
5. 关系分析（分析沟通模式和关系，200 字内）
6. 常聊话题（提取三个常聊话题）
7. 一句话总结（风趣犀利，像江湖传说）
```

## 安全说明

- 🚫 **数据不上传**：聊天记录数据仅在本地处理，不会上传到任何第三方服务器
- 🔒 **隐私保护**：所有操作都在本地完成，不存储任何敏感数据
- 📝 **用户可控**：用户完全控制数据处理流程

## 兼容性

- ✅ wechat-export v0.1.0+ 导出的 JSON 文件
- ✅ Go 1.25+
- ✅ macOS / Linux / Windows

## 常见问题

**Q: 必须用 wechat-export 导出的 JSON 吗？**
A: 是的，该工具仅支持解析 wechat-export 导出的特定 JSON 格式。

**Q: 输出的 HTML 报告在哪里？**
A: 默认会生成在 `./wechat_analyze_stats` 或 `./wechat_analyze_ai` 目录下。

**Q: AI 分析需要联网吗？**
A: 需要，AI 分析需要调用大模型 API。统计分析和 HTML 生成无需联网。

**Q: 支持分析群聊记录吗？**
A: 目前仅支持分析单人对话记录，群聊记录分析在规划中。

## License

MIT
