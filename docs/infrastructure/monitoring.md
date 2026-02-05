# Monitoring & Analytics

FitGlue uses a GCP-native observability stack with Sentry for error tracking. This guide explains how to access dashboards, configure alerts, and set up business analytics.

## Quick Links

| Dashboard | URL | Purpose |
|-----------|-----|---------|
| **Cloud Monitoring** | [GCP Console → Monitoring](https://console.cloud.google.com/monitoring) | System health, function metrics |
| **Cloud Logging** | [GCP Console → Logging](https://console.cloud.google.com/logs) | Structured logs, debugging |
| **BigQuery** | [GCP Console → BigQuery](https://console.cloud.google.com/bigquery) | Historical analytics |
| **Looker Studio** | [lookerstudio.google.com](https://lookerstudio.google.com) | Business dashboards |
| **Sentry** | [fitglue.sentry.io](https://fitglue.sentry.io) | Error tracking |

---

## GCP Cloud Monitoring Dashboards

After deploying `terraform/monitoring.tf`, these dashboards are available in GCP Console → Monitoring → Dashboards:

| Dashboard | Metrics |
|-----------|---------|
| **FitGlue Operations Overview** | Total invocations, error rate, latency (p50/p95), Firestore read/writes |
| **FitGlue Provider API Latency** | Uploader execution times by provider (Strava, TrainingPeaks, Intervals) |
| **FitGlue Handler Performance** | Per-handler invocations and latency grouped by category |
| **FitGlue Enricher Performance** | Booster provider executions, latency, success/failure rates |
| **FitGlue Business Growth** | Activity trends, source distribution, destination success rates |

### Alert Policies

| Alert | Condition | Notification |
|-------|-----------|--------------|
| High Error Rate | Any function > 5% errors in 5 min | Email |
| Critical Function Failure | Pipeline function > 5 errors in 1 min | Email |
| High Latency | Any function p95 > 30 seconds | Email |

**To configure alerts email:**
1. Edit `terraform/monitoring.tf`
2. Update `alerts@fitglue.com` in `google_monitoring_notification_channel.email`
3. Run `terraform apply`

---

## BigQuery Analytics

The `fitglue_analytics` dataset (deployed via `terraform/analytics.tf`) contains:

### Pre-Built Views

| View | Description | Use For |
|------|-------------|---------|
| `v_pipeline_summary` | Daily success/fail/skip counts | Pipeline health trends |
| `v_enricher_popularity` | Provider execution counts & latency | Booster usage analytics |
| `v_daily_active_users` | Unique users per day | Growth tracking |
| `v_weekly_growth` | Week-over-week comparison | Growth trends |
| `v_destination_success` | Upload success rates by provider | Reliability tracking |
| `v_source_distribution` | Activities by source integration | Integration breakdown |
| `v_executive_summary` | Today/week/month KPIs | Executive reporting |

### Accessing BigQuery

```bash
# Open BigQuery Console
open "https://console.cloud.google.com/bigquery?project=YOUR_PROJECT_ID&d=fitglue_analytics"

# Example query: Top enrichers this week
bq query --use_legacy_sql=false '
SELECT provider_name, SUM(execution_count) as total
FROM `fitglue_analytics.v_enricher_popularity`
WHERE date >= DATE_SUB(CURRENT_DATE(), INTERVAL 7 DAY)
GROUP BY provider_name
ORDER BY total DESC
LIMIT 10'
```

---

## Looker Studio Setup

1. Go to [lookerstudio.google.com](https://lookerstudio.google.com)
2. Create a **Blank Report**
3. Select **BigQuery** as data source
4. Connect to `YOUR_PROJECT_ID.fitglue_analytics.v_*` views

### Suggested Dashboards

**Growth Dashboard:**
- Time series: Daily active users
- Scorecard: Total activities this week
- Trend arrow: Week-over-week growth

**Enricher Dashboard:**
- Bar chart: Provider popularity
- Line chart: Provider latency over time
- Table: Top providers by execution count

---

## Stripe Integration (Revenue Analytics)

To add revenue metrics to BigQuery:

### Option 1: Stripe Data Pipeline (Recommended)

1. Log in to [Stripe Dashboard](https://dashboard.stripe.com)
2. Go to **Developers → Data Pipeline**
3. Click **Create pipeline**
4. Select **BigQuery** as destination
5. Authenticate with your GCP project
6. Select data to sync: `subscriptions`, `invoices`, `customers`
7. Choose destination dataset: `fitglue_analytics`

**Cost:** Free from Stripe, ~$1-5/month BigQuery storage

### Option 2: Manual Export

```bash
# Export Stripe data via CLI
stripe invoices list --limit 100 | jq > invoices.json

# Load into BigQuery
bq load --source_format=NEWLINE_DELIMITED_JSON \
  fitglue_analytics.stripe_invoices invoices.json
```

### Revenue Queries

Once Stripe data is in BigQuery:

```sql
-- Monthly Recurring Revenue
SELECT 
  DATE_TRUNC(created, MONTH) as month,
  SUM(amount_paid / 100) as mrr_usd
FROM `fitglue_analytics.stripe_invoices`
WHERE status = 'paid'
GROUP BY month
ORDER BY month DESC;

-- Trial Conversion Rate
SELECT 
  COUNT(CASE WHEN subscription_status = 'active' THEN 1 END) * 100.0 / COUNT(*) as conversion_rate
FROM `fitglue_analytics.stripe_subscriptions`
WHERE trial_end IS NOT NULL;
```

---

## Sentry Configuration

Sentry is configured via environment variables and Terraform secrets. See `terraform/secrets.tf` for DSN configuration.

### Recommended Settings (Cost Optimization)

| Setting | Dev | Prod |
|---------|-----|------|
| Error sample rate | 1.0 | 1.0 |
| Performance sample rate | 0.1 | 0.05 |
| Session tracking | Enabled | Disabled |

**Free tier limits:** 5K errors/month, 10K transactions/month

---

## Troubleshooting

### No data in dashboards?
1. Ensure `terraform apply` was run after creating monitoring.tf and analytics.tf
2. Wait 5-10 minutes for metrics to propagate
3. Check Cloud Logging for recent function executions

### BigQuery views return empty?
Log-based metrics only capture data from log entries *after* the metric was created. Historical data is not backfilled.

### Alerts not triggering?
1. Verify notification channel email is correct
2. Check that alert policies are enabled in Cloud Monitoring → Alerting
