const fs = require("fs");
const path = require("path");
const { PassThrough } = require("stream");

const DEFAULT_SOURCE_DIR = process.env.SOURCE_DIR || "/vol1/1000";
const SOURCE_MAP = {
  personal: { dir: "/vol1/1000", label: "personal" },
  team: { dir: "/vol1/@team", label: "team" },
};

function normalizeBaseUrl(baseUrl) {
  return String(baseUrl || "").trim().replace(/\/+$/, "");
}

function normalizeStorage(storage) {
  return String(storage || "").trim().replace(/^\/+|\/+$/g, "");
}

async function readJson(res) {
  try {
    return await res.json();
  } catch {
    return null;
  }
}

function assertResponse(res, data, actionLabel) {
  if (!res.ok) {
    throw new Error(`${actionLabel}失败(${res.status})`);
  }

  if (data && Object.prototype.hasOwnProperty.call(data, "success")) {
    if (data.success !== true && data.success !== 200) {
      throw new Error(data.message || data.msg || `${actionLabel}失败`);
    }
  }

  if (data && Object.prototype.hasOwnProperty.call(data, "code")) {
    if (data.code !== 200) {
      throw new Error(data.message || data.msg || `${actionLabel}失败`);
    }
  }
}

function resolveSource(sourceType) {
  if (sourceType && SOURCE_MAP[sourceType]) {
    return SOURCE_MAP[sourceType];
  }

  if (sourceType) {
    return null;
  }

  if (DEFAULT_SOURCE_DIR === SOURCE_MAP.team.dir) {
    return SOURCE_MAP.team;
  }

  if (DEFAULT_SOURCE_DIR === SOURCE_MAP.personal.dir) {
    return SOURCE_MAP.personal;
  }

  return { dir: DEFAULT_SOURCE_DIR, label: "custom" };
}

function toPosixPath(input) {
  return String(input || "").split(path.sep).join("/");
}

function isDirExistsMessage(message) {
  const msg = String(message || "").toLowerCase();
  return msg.includes("exist") || msg.includes("exists") || msg.includes("已存在") || msg.includes("already");
}

function sendEvent(stream, type, data) {
  const payload = data ? JSON.stringify(data) : "";
  stream.write(`event: ${type}\n`);
  stream.write(`data: ${payload}\n\n`);
}

async function login(baseUrl, username, password) {
  const loginRes = await fetch(`${baseUrl}/v1/users/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  const loginData = await readJson(loginRes);
  assertResponse(loginRes, loginData, "登录");

  const token =
    loginData?.data?.token?.access_token ||
    loginData?.data?.token ||
    loginData?.token ||
    "";

  if (!token) {
    throw new Error("登录成功但未获取到 token");
  }

  return token;
}

async function scanSourceDir(rootDir) {
  const files = [];
  const dirs = [];
  const stack = [""];

  while (stack.length) {
    const relDir = stack.pop();
    const absDir = path.join(rootDir, relDir);
    const entries = await fs.promises.readdir(absDir, { withFileTypes: true });

    for (const entry of entries) {
      const relPath = path.join(relDir, entry.name);
      if (entry.isDirectory()) {
        dirs.push(relPath);
        stack.push(relPath);
      } else if (entry.isFile()) {
        files.push(relPath);
      }
    }
  }

  return { files, dirs };
}

async function ensureRemoteDir({ baseUrl, token, dirPath, supportsMkdir }) {
  if (!supportsMkdir.value || !dirPath) {
    return;
  }

  const res = await fetch(`${baseUrl}/v2_1/files/folder`, {
    method: "POST",
    headers: {
      Authorization: token,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ path: dirPath }),
  });
  const data = await readJson(res);

  if (res.status === 404) {
    supportsMkdir.value = false;
    return;
  }

  if (!res.ok) {
    const message = data?.message || data?.msg || "";
    if (isDirExistsMessage(message)) {
      return;
    }
    throw new Error(message || `创建目录失败(${dirPath})`);
  }

  if (data && Object.prototype.hasOwnProperty.call(data, "success")) {
    if (data.success !== true && data.success !== 200) {
      if (!isDirExistsMessage(data.message || data.msg)) {
        throw new Error(data.message || data.msg || `创建目录失败(${dirPath})`);
      }
    }
  }

  if (data && Object.prototype.hasOwnProperty.call(data, "code")) {
    if (data.code !== 200) {
      if (!isDirExistsMessage(data.message || data.msg)) {
        throw new Error(data.message || data.msg || `创建目录失败(${dirPath})`);
      }
    }
  }
}

async function uploadFile({ baseUrl, token, remoteDir, filename, fullPath }) {
  const [fileBuffer, stat] = await Promise.all([
    fs.promises.readFile(fullPath),
    fs.promises.stat(fullPath),
  ]);
  const formData = new FormData();
  formData.append("path", remoteDir);
  formData.append("modTime", Math.floor(stat.mtimeMs / 1000).toString());
  formData.append(
    "file",
    new Blob([fileBuffer], { type: "application/octet-stream" }),
    filename
  );

  const uploadRes = await fetch(`${baseUrl}/v2_1/files/file/uploadV2`, {
    method: "POST",
    headers: { Authorization: token },
    body: formData,
  });
  const uploadData = await readJson(uploadRes);
  assertResponse(uploadRes, uploadData, "上传");
}

async function runMigration(stream, options) {
  const { baseUrl, username, password, storagePath, sourceInfo } = options;
  const dstPath = storagePath;
  let currentStep = "login";

  try {
    sendEvent(stream, "progress", {
      step: "login",
      status: "start",
      message: "正在登录 ZimaOS...",
    });
    const token = await login(baseUrl, username, password);
    sendEvent(stream, "progress", {
      step: "login",
      status: "success",
      message: "登录成功",
    });

    currentStep = "scan";
    sendEvent(stream, "progress", {
      step: "scan",
      status: "start",
      message: "正在扫描目录...",
    });
    const { files, dirs } = await scanSourceDir(sourceInfo.dir);
    const totalFiles = files.length;
    sendEvent(stream, "progress", {
      step: "scan",
      status: "success",
      message: `扫描完成：${totalFiles} 个文件`,
      totalFiles,
    });

    currentStep = "upload";
    const supportsMkdir = { value: true };
    const createdDirs = new Set();
    const sortedDirs = dirs.map(toPosixPath).sort((a, b) => a.length - b.length);

    for (const relDir of sortedDirs) {
      if (!relDir || createdDirs.has(relDir)) {
        continue;
      }
      const remoteDir = path.posix.join(dstPath, relDir);
      await ensureRemoteDir({ baseUrl, token, dirPath: remoteDir, supportsMkdir });
      createdDirs.add(relDir);
    }

    sendEvent(stream, "progress", {
      step: "upload",
      status: "start",
      message: totalFiles ? "开始上传文件..." : "无需上传文件",
      totalFiles,
    });

    for (let index = 0; index < files.length; index += 1) {
      const relPath = files[index];
      const relPosix = toPosixPath(relPath);
      const fullPath = path.join(sourceInfo.dir, relPath);
      const dirName = path.posix.dirname(relPosix);
      const remoteDir = dirName === "." ? dstPath : path.posix.join(dstPath, dirName);
      const filename = path.posix.basename(relPosix);

      sendEvent(stream, "progress", {
        step: "upload",
        status: "start",
        message: `正在上传 ${index + 1}/${totalFiles}`,
        currentFile: relPosix,
        transferredFiles: index,
        totalFiles,
      });

      await uploadFile({ baseUrl, token, remoteDir, filename, fullPath });
      sendEvent(stream, "progress", {
        step: "upload",
        status: "start",
        message: `正在上传 ${index + 1}/${totalFiles}`,
        currentFile: relPosix,
        transferredFiles: index + 1,
        totalFiles,
      });
    }

    sendEvent(stream, "progress", {
      step: "upload",
      status: "success",
      message: "文件上传完成",
      transferredFiles: totalFiles,
      totalFiles,
    });

    sendEvent(stream, "done", {
      message: "迁移完成",
      result: {
        dstPath,
        sourceDir: sourceInfo.dir,
        sourceType: sourceInfo.label,
        totalFiles,
      },
    });
  } catch (error) {
    error.step = currentStep;
    throw error;
  }
}

module.exports = async function migrate({ body }) {
  if (typeof fetch !== "function") {
    return { code: 500, msg: "当前环境不支持原生 fetch", data: null };
  }

  if (typeof FormData === "undefined" || typeof Blob === "undefined") {
    return { code: 500, msg: "当前环境缺少 FormData/Blob", data: null };
  }

  const baseUrl = normalizeBaseUrl(body.baseUrl);
  const username = String(body.username || "").trim();
  const password = String(body.password || "");
  const storage = normalizeStorage(body.storage);
  const sourceType = String(body.source || body.space || "").trim();

  if (!baseUrl || !username || !password) {
    return { code: 400, msg: "缺少 baseUrl/username/password", data: body };
  }

  const sourceInfo = resolveSource(sourceType);
  if (!sourceInfo) {
    return { code: 400, msg: "未知的迁移空间", data: body };
  }

  if (!fs.existsSync(sourceInfo.dir)) {
    return { code: 400, msg: "源目录不存在", data: null };
  }

  const sourceStat = fs.statSync(sourceInfo.dir);
  if (!sourceStat.isDirectory()) {
    return { code: 400, msg: "源路径不是目录", data: null };
  }

  const storagePath = storage ? `/media/${storage}` : "/media";
  const stream = new PassThrough();

  setImmediate(() => {
    runMigration(stream, {
      baseUrl,
      username,
      password,
      storagePath,
      sourceInfo,
    })
      .catch((error) => {
        sendEvent(stream, "error", {
          step: error?.step,
          message: error?.message || "迁移失败",
        });
      })
      .finally(() => {
        stream.end();
      });
  });

  return {
    code: 200,
    type: "text/event-stream; charset=utf-8",
    headers: {
      "Cache-Control": "no-cache",
      Connection: "keep-alive",
      "X-Accel-Buffering": "no",
    },
    stream,
  };
};
