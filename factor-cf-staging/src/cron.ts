/** Fly crontab jobs in America/New_York, expressed as UTC cron strings (EDT, UTC−4). */
export const CRON_JOBS: { cron: string; path: string; label: string }[] = [
	{
		cron: "30 12 * * 1-5",
		path: "/internal/cron/sendSavedStrategySummaryEmails",
		label: "sendSavedStrategySummaryEmails",
	},
	{
		cron: "0 13 * * 1-5",
		path: "/internal/cron/updatePrices",
		label: "updatePrices",
	},
	{
		cron: "0 14 * * 1-5",
		path: "/internal/cron/rebalance",
		label: "rebalance",
	},
	{
		cron: "*/15 14-20 * * 1-5",
		path: "/internal/cron/updateOrders",
		label: "updateOrders",
	},
];

export function cronPathForExpression(cronExpr: string): string | undefined {
	return CRON_JOBS.find((j) => j.cron === cronExpr)?.path;
}
