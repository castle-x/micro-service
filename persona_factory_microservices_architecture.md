# Persona Factory 微服务架构方案（最新版）

## 一、系统本质

这不是一个传统的自动化 Workflow 系统。

本质上它是：

# Guided Interactive Process（引导式交互流程）

也就是：

```text
下一步 / 上一步 / 重试 / 恢复
```

用户逐步完成角色创建与内容生产。

核心模式：

```text
Human-in-the-loop
+
Stateful Wizard Flow
+
LLM + Image Generation
```

核心原则：

```text
Character is the Asset
Prompt is the Compiler Output
Workflow is the State Machine
```

---

# 二、两套核心 Workflow

必须明确区分：

---

## Workflow A：Character Creation（角色创建）

目标：

```text
创建可长期复用的角色资产
```

流程：

```text
Persona Card
↓
Face Lock
↓
Body Lock
↓
Style Lock
↓
Reference Pool
```

特点：

```text
低频
重流程
强人工确认
长期复用
```

---

## Workflow B：Content Production（内容生产）

目标：

```text
基于已有角色快速批量产出内容图
```

流程：

```text
Select Persona
↓
Select Goal
↓
Select Content Package
↓
Batch Generate
↓
Human Selection
↓
Refine
↓
Publish
```

特点：

```text
高频
轻流程
批量抽卡
持续产出
```

这是日常核心流程。

---

# 三、核心微服务

---

## 1. Persona Service（角色资产服务）

### 职责

管理长期角色资产：

- Persona Card
- Visual DNA
- Reference Pool
- Locked State
- Persona Version
- Hero Face / Hero Body
- Best Performing Images

### 管什么

```text
她是谁
```

---

## 2. Workflow Service（流程状态服务）

### 本质

不是 Workflow Engine

而是：

# Stateful Context Manager

### 职责

负责：

```text
当前在哪一步
是否允许进入下一步
上一步是否完成
是否允许回退
是否允许重试
断点续跑
步骤快照保存
上下文组装
```

### 管什么

```text
现在该做什么
```

---

## 3. Prompt Orchestration Service（提示词编排服务）

### 职责

负责每个节点的执行计划：

- Node Plan
- Structured Decision
- Context Injection
- Prompt Compiler
- Rule Engine
- LLM Planner

### 管什么

```text
这一节点应该怎么做
```

### 核心不是

```text
拼 Prompt
```

而是：

```text
Decision → Plan → Prompt
```

---

## 4. Inference Job Service（生成任务服务）

### 职责

统一管理单次模型调用：

- 创建任务
- 排队执行
- 重试失败
- 扣费 / 结算
- 状态管理
- 回调处理
- 保存结果

### 管什么

```text
真正去执行模型调用
```

这是短生命周期任务。

---

## 5. Model Gateway Service（模型网关服务）

### 职责

统一适配外部模型：

- OpenAI Image
- Gemini Image
- Seedream
- fal.ai
- Replicate

统一接口：

```ts
generate()
edit()
upscale()
```

### 管什么

```text
跟模型厂商打交道
```

---

## 6. Asset Service（图片资产服务）

### 职责

管理图片资产：

- 上传参考图
- 保存生成图
- 原图 / 缩略图
- Asset Role
- 图片版本
- 元数据

### 管什么

```text
图片在哪里
```

---

# 四、最核心的关系

---

## Workflow → Node → Job

```text
一个 Workflow
  包含多个 Step / Node

一个 Node
  可产生多个 Inference Job

一个 Inference Job
  只代表一次模型调用
```

例如：

```text
Face Lock Node
  ├─ Job 1：生成 8 张脸（失败）
  ├─ Job 2：重新生成 8 张（成功）
  └─ Job 3：根据反馈再生成 4 张（成功）
```

最终：

```text
Face Lock = Approved
```

然后进入：

```text
Body Lock
```

---

# 五、Workflow 本质

本质不是自动执行。

而是：

# Step Validation + State Persistence + Context Assembly

也就是：

```text
前端：下一步 / 上一步

后端：
校验 → 执行 → 保存 → 返回下一状态
```

结束。

---

# 六、核心接口（MVP）

只需要 5 个：

---

## 1. start()

```http
POST /workflow/start
```

创建流程实例。

---

## 2. next()

```http
POST /workflow/next-step
```

进入下一步。

核心接口。

---

## 3. back()

```http
POST /workflow/back-step
```

回退上一步。

---

## 4. retry()

```http
POST /workflow/retry-step
```

重试当前步骤。

例如：

```text
再生成 4 张图
```

---

## 5. resume()

```http
GET /workflow/detail
```

恢复流程。

例如：

```text
昨天做到哪了
```

---

# 七、Step Snapshot（最重要数据）

真正值钱的数据不是 Prompt。

而是：

# Step Snapshot

```ts
StepSnapshot {
  workflowId
  stepType

  inputContext
  nodePlan

  candidateResults
  selectedDecision

  selectedAssets
  userFeedback

  status
}
```

它决定：

```text
可恢复
可回退
可重试
可追溯
可继承
```

---

# 八、Node Plan 与 Node Output

---

## Node Plan（系统内部）

```json
{
  "goal": "create hero face",
  "promptBlocks": [],
  "referenceImages": [],
  "negativePrompt": "",
  "modelProvider": "OpenAI",
  "validationRules": []
}
```

作用：

```text
给系统执行
```

---

## Node Output（用户可见）

例如：

```text
8 张候选脸图
结构化角色卡
服装方向选择
最终成品图
```

作用：

```text
给用户做决策
```

---

# 九、Prompt 的真正位置

不要保存 Prompt。

应该保存：

```text
Structured Decision
+
Reference Asset
+
Locked State
```

因为：

```text
Prompt 是临时执行语言
Character Memory 才是核心资产
```

最终流程：

```text
Structured JSON
↓
Prompt Compiler
↓
Prompt
↓
Image Model
```

不是：

```text
JSON 直接喂给生图模型
```

---

# 十、完整调用链路（标准版）

```text
User Intent
↓
Workflow Service
↓
Prompt Orchestration Service
↓
Inference Job Service
↓
Model Gateway Service
↓
External Models
↓
Asset Service + Persona Service
↓
Workflow State Update
↓
Next Step
```

---

# 十一、一句话总结

```text
Workflow 决定：做什么

Prompt 决定：怎么做

Job 决定：去执行

Gateway 决定：跟谁做

Persona + Asset 决定：沉淀什么
```

你的系统不是：

```text
自动帮用户完成工作
```

而是：

# 引导用户做出最赚钱的选择

这才是 Persona Factory 的完整后端骨架。

