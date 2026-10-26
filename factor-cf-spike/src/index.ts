// Container APIs are newer than published workers-types; wrangler deploy is the source of truth.
// @ts-nocheck

import { DurableObject } from "cloudflare:workers";

export interface Env {
	SPIKE_CONTAINER: DurableObjectNamespace<SpikeContainer>;
}

/** Starts the spike container on cron or on-demand HTTP for cold-start measurement. */
export class SpikeContainer extends DurableObject<Env> {
	private currentRun: Promise<void> | undefined;

	async fetch(request: Request): Promise<Response> {
		const url = new URL(request.url);
		if (url.pathname === "/do/health") {
			return this.serveHealth();
		}
		if (url.pathname === "/do/cron") {
			const reason = url.searchParams.get("reason") ?? "cron:manual";
			await this.run(reason);
			return Response.json({ ok: true, reason });
		}
		return new Response("not found", { status: 404 });
	}

	run(reason: string): Promise<void> {
		this.currentRun ??= this.runOnce(reason).finally(() => {
			this.currentRun = undefined;
		});
		return this.currentRun;
	}

	private async runOnce(reason: string): Promise<void> {
		const container = this.ctx.container!;
		if (!container.running) {
			container.start({
				image: container.images.base,
				instance: "lite",
				enableInternet: true,
				env: { REASON: reason },
			});
		}
		await container.monitor();
	}

	private async serveHealth(): Promise<Response> {
		const container = this.ctx.container!;
		if (!container.running) {
			container.start({
				image: container.images.base,
				instance: "lite",
				enableInternet: true,
				env: { REASON: "http-health" },
			});
		}
		const url = "http://container/health";
		let lastError: unknown;
		for (let attempt = 0; attempt < 40; attempt++) {
			try {
				const res = await container.fetch(new Request(url));
				if (res.ok) {
					return res;
				}
				lastError = new Error(`container health status ${res.status}`);
			} catch (err) {
				lastError = err;
			}
			await new Promise((r) => setTimeout(r, 250));
		}
		return Response.json(
			{ ok: false, error: String(lastError) },
			{ status: 503 },
		);
	}
}

export default {
	fetch(request: Request, env: Env): Promise<Response> {
		const url = new URL(request.url);
		const id = env.SPIKE_CONTAINER.idFromName("spike");
		const stub = env.SPIKE_CONTAINER.get(id);
		if (url.pathname === "/health") {
			return stub.fetch(new Request("http://do/do/health"));
		}
		return Promise.resolve(
			new Response(
				"factor-cf-spike: GET /health cold-starts the container; cron runs via scheduled().",
				{ headers: { "content-type": "text/plain; charset=utf-8" } },
			),
		);
	},

	async scheduled(controller: ScheduledController, env: Env): Promise<void> {
		const id = env.SPIKE_CONTAINER.idFromName("spike");
		const stub = env.SPIKE_CONTAINER.get(id);
		await stub.run(
			`cron:${new Date(controller.scheduledTime).toISOString()}`,
		);
	},
} satisfies ExportedHandler<Env>;
