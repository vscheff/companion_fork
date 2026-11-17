import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import { test } from "node:test";

// Node's type stripping needs explicit extensions for the Worker's bundler imports.
const hooks = registerHooks({
  resolve(specifier, context, nextResolve) {
    if (context.parentURL?.endsWith(".ts") && /^\.\/[\w-]+$/.test(specifier)) {
      return nextResolve(specifier + ".ts", context);
    }
    return nextResolve(specifier, context);
  },
});
const { default: worker } = await import("../src/index.ts");
hooks.deregister();

function registrationEnv(inserts) {
  return {
    IP_LIMITER: { limit: async () => ({ success: true }) },
    REGISTRATION_LIMITER: { limit: async () => ({ success: true }) },
    DB: {
      prepare(sql) {
        assert.match(sql, /^INSERT INTO devices/);
        assert.doesNotMatch(sql, /ip/i);
        return { bind: (...values) => ({ run: async () => inserts.push(values) }) };
      },
    },
  };
}

function register(body, env, headers) {
  const req = new Request("https://worker.test/v1/devices", {
    method: "POST",
    body,
    headers,
    duplex: "half",
  });
  return worker.fetch(req, env, {});
}

async function assertError(response, status, error) {
  assert.equal(response.status, status);
  assert.equal(response.headers.get("content-type"), "application/json");
  assert.deepEqual(await response.json(), { error });
}

test("registers a device using the desktop client's fields", async () => {
  const inserts = [];
  const clientId = "a".repeat(32);
  const body = JSON.stringify({ clientId, appVersion: "0.1.12", os: "windows" });
  const response = await register(body, registrationEnv(inserts));

  assert.equal(response.status, 201);
  const { deviceId, token } = await response.json();
  assert.match(deviceId, /^dev_[0-9a-f]{20}$/);
  assert.match(token, /^[A-Za-z0-9_-]{43}$/);
  assert.equal(inserts.length, 1);
  assert.equal(inserts[0][0], deviceId);
  assert.equal(inserts[0][2], "companion 0.1.12");
  assert.equal(inserts[0].length, 5);
  assert.equal(inserts[0][4], clientId);
});

test("accepts an object with optional fields omitted at the 4 KiB limit", async () => {
  const inserts = [];
  const body = "{}".padEnd(4096);
  const response = await register(body, registrationEnv(inserts));
  assert.equal(response.status, 201);
  assert.equal(inserts.length, 1);
  assert.equal(inserts[0][2], "companion");
  assert.equal(inserts[0][4], null);
});

for (const [name, body] of [
  ["missing body", undefined],
  ["malformed JSON", "{"],
  ["null", "null"],
  ["array", "[]"],
  ["string", '"value"'],
  ["number", "42"],
  ["boolean", "true"],
]) {
  test(`rejects ${name} without creating a device`, async () => {
    const inserts = [];
    const response = await register(body, registrationEnv(inserts));
    await assertError(response, 400, "Registration body must be a JSON object");
    assert.deepEqual(inserts, []);
  });
}

for (const contentLength of [undefined, "4097", "10"]) {
  test(`stops reading oversized bodies with Content-Length ${contentLength ?? "omitted"}`, { timeout: 2000 }, async () => {
    const inserts = [];
    const headers = contentLength === undefined ? {} : { "Content-Length": contentLength };
    const encoder = new TextEncoder();
    let canceled = false;
    const body = new ReadableStream({
      start(controller) {
        // The JSON is under 4 KiB in characters, but 4,097 bytes in UTF-8.
        controller.enqueue(encoder.encode('{"pad":"' + "é".repeat(1020)));
        controller.enqueue(encoder.encode("é".repeat(1023) + '"} '));
      },
      cancel() {
        canceled = true;
      },
    });

    const response = await register(body, registrationEnv(inserts), headers);
    await assertError(response, 413, "Registration body too large");
    assert.equal(canceled, true);
    assert.deepEqual(inserts, []);
  });
}

test("rejects registrations over the network rate limit without storing anything", async () => {
  const inserts = [];
  const env = registrationEnv(inserts);
  env.REGISTRATION_LIMITER = { limit: async () => ({ success: false }) };
  const response = await register("{}", env);
  await assertError(response, 429, "Too many registrations from this network");
  assert.deepEqual(inserts, []);
});

test("returns a generic 500 after an asynchronous registration failure", async (t) => {
  t.mock.method(console, "error", () => {});
  const env = registrationEnv([]);
  env.DB = {
    prepare: () => ({
      bind: () => ({ run: () => Promise.reject(new Error("private database details")) }),
    }),
  };
  const response = await register("{}", env);
  await assertError(response, 500, "Internal error");
});

test("returns a generic 500 when an upload handler encounters an exception", async (t) => {
  t.mock.method(console, "error", () => {});
  const env = {
    IP_LIMITER: { limit: () => { throw new Error("private binding details"); } },
  };
  const req = new Request("https://worker.test/v1/uploads", { method: "PUT" });
  const response = await worker.fetch(req, env, {});
  await assertError(response, 500, "Internal error");
});
