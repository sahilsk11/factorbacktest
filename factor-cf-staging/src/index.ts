import { Container, getContainer } from "@cloudflare/containers";
import { cronPathForExpression } from "./cron";
import { factorContainerEnv, type Env } from "./env";

export class FactorApiContainer extends Container<Env> {
	defaultPort = 3009;
	sleepAfter = "5m";
	enableInternet = true;
	pingEndpoint = "/";

	constructor(ctx: DurableObjectState, env: Env) {
		super(ctx as DurableObjectState<Env>, env);
		this.envVars = factorContainerEnv(env);
	}
}

function apiContainer(env: Env) {
	const ns = env.FACTOR_API as unknown as DurableObjectNamespace<FactorApiContainer>;
	return getContainer(ns, "primary");
}

async function proxyToApi(request: Request, env: Env): Promise<Response> {
	return apiContainer(env).fetch(request);
}

async function runCron(env: Env, path: string): Promise<Response> {
	if (!env.CRON_SECRET) {
		return new Response("CRON_SECRET not configured on Worker", { status: 500 });
	}
	const url = new URL(path, "http://container");
	return apiContainer(env).fetch(
		new Request(url, {
			method: "POST",
			headers: { "X-Cron-Secret": env.CRON_SECRET },
		}),
	);
}

export default {
	fetch(request: Request, env: Env): Promise<Response> {
		return proxyToApi(request, env);
	},

	async scheduled(
		controller: ScheduledController,
		env: Env,
	): Promise<void> {
		const path = cronPathForExpression(controller.cron);
		if (!path) {
			console.error("unknown cron expression", controller.cron);
			return;
		}
		const res = await runCron(env, path);
		if (!res.ok) {
			const body = await res.text();
			console.error("cron failed", path, res.status, body.slice(0, 500));
		}
	},
} satisfies ExportedHandler<Env>;
