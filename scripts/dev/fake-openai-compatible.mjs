#!/usr/bin/env node
import http from "node:http";

function argValue(name, fallback) {
  const index = process.argv.indexOf(`--${name}`);
  if (index >= 0 && process.argv[index + 1]) {
    return process.argv[index + 1];
  }
  return fallback;
}

const port = Number(argValue("port", "39090"));
const key = argValue("key", "fake-key");

function writeJSON(res, status, body) {
  res.writeHead(status, { "content-type": "application/json" });
  res.end(JSON.stringify(body));
}

async function readJSON(req) {
  const chunks = [];
  for await (const chunk of req) {
    chunks.push(chunk);
  }
  const raw = Buffer.concat(chunks).toString("utf8");
  return raw ? JSON.parse(raw) : {};
}

function chatCompletion(body) {
  if (Array.isArray(body.tools) && body.tools.length > 0) {
    const fn = body.tools[0]?.function?.name || "fake_tool";
    return {
      id: "chatcmpl-fake",
      object: "chat.completion",
      choices: [
        {
          index: 0,
          message: {
            role: "assistant",
            content: "",
            tool_calls: [
              {
                id: "call_fake_1",
                type: "function",
                function: { name: fn, arguments: "{\"ok\":true}" },
              },
            ],
          },
          finish_reason: "tool_calls",
        },
      ],
      usage: { prompt_tokens: 6, completion_tokens: 4, total_tokens: 10 },
    };
  }
  return {
    id: "chatcmpl-fake",
    object: "chat.completion",
    choices: [
      {
        index: 0,
        message: { role: "assistant", content: "fake upstream response" },
        finish_reason: "stop",
      },
    ],
    usage: { prompt_tokens: 5, completion_tokens: 3, total_tokens: 8 },
  };
}

function writeSSE(res, payload) {
  res.write(`data: ${JSON.stringify(payload)}\n\n`);
}

function streamChatCompletion(res, body) {
  res.writeHead(200, {
    "content-type": "text/event-stream",
    "cache-control": "no-cache",
    connection: "keep-alive",
  });

  if (Array.isArray(body.tools) && body.tools.length > 0) {
    const fn = body.tools[0]?.function?.name || "fake_tool";
    writeSSE(res, {
      id: "chatcmpl-fake",
      object: "chat.completion.chunk",
      choices: [
        {
          index: 0,
          delta: {
            tool_calls: [
              {
                index: 0,
                id: "call_fake_1",
                type: "function",
                function: { name: fn, arguments: "{\"ok\"" },
              },
            ],
          },
          finish_reason: null,
        },
      ],
    });
    writeSSE(res, {
      id: "chatcmpl-fake",
      object: "chat.completion.chunk",
      choices: [
        {
          index: 0,
          delta: {
            tool_calls: [
              {
                index: 0,
                function: { arguments: ":true}" },
              },
            ],
          },
          finish_reason: "tool_calls",
        },
      ],
    });
  } else {
    writeSSE(res, {
      id: "chatcmpl-fake",
      object: "chat.completion.chunk",
      choices: [{ index: 0, delta: { content: "fake upstream " }, finish_reason: null }],
    });
    writeSSE(res, {
      id: "chatcmpl-fake",
      object: "chat.completion.chunk",
      choices: [{ index: 0, delta: { content: "response" }, finish_reason: "stop" }],
    });
  }

  writeSSE(res, {
    id: "chatcmpl-fake",
    object: "chat.completion.chunk",
    choices: [],
    usage: { prompt_tokens: 5, completion_tokens: 3, total_tokens: 8 },
  });
  res.end("data: [DONE]\n\n");
}

const server = http.createServer(async (req, res) => {
  const isChatCompletion = req.url === "/chat/completions" || req.url === "/v1/chat/completions";
  if (req.method !== "POST" || !isChatCompletion) {
    writeJSON(res, 404, { error: { message: "not found" } });
    return;
  }
  if (req.headers.authorization !== `Bearer ${key}`) {
    writeJSON(res, 401, { error: { message: "invalid api key" } });
    return;
  }
  let body;
  try {
    body = await readJSON(req);
  } catch {
    writeJSON(res, 400, { error: { message: "invalid JSON body" } });
    return;
  }
  if (body.stream === true) {
    streamChatCompletion(res, body);
    return;
  }
  writeJSON(res, 200, chatCompletion(body));
});

server.listen(port, () => {
  console.log(`fake OpenAI-compatible upstream listening on http://127.0.0.1:${port}`);
});
