const Koa = require("koa");
const bodyParser = require("koa-bodyparser");
const cors = require("@koa/cors");

const exec = require("./utils/exec");

const app = new Koa();

app.use(cors());

app.use(bodyParser());

app.use(async (ctx) => {
  const apiFromPath = ctx.path === "/migrate" ? "migrate" : "";
  const data = {
    api: ctx.request.headers["api-path"] || apiFromPath,
    query: ctx.query || {},
    body: ctx.request.body || {},
  };

  const { type, body } = await exec(data);

  if (type && body.stream) {
    ctx.set("Content-Type", type);
    if (body.headers) {
      Object.entries(body.headers).forEach(([key, value]) => {
        if (value !== undefined) {
          ctx.set(key, String(value));
        }
      });
    }
    if (body.size) {
      ctx.set("Content-Length", body.size);
    }
    ctx.body = body.stream;
  } else {
    ctx.set("Content-Type", "application/json; charset=utf-8");
    ctx.body = body;
  }
});

app.listen(17746);
