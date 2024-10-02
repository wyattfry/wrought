# Wrought

This program will fetch a Gitlab user's events from Gitlab or a file for a given timeframe, then use ChatGPT to summarize the events into bullet points. Requires a paid OpenAI account, or an account with a credit balance.

```
Usage of wrought:
  -count int
        Number of bullet points for summary (default 3)
  -end string
        End date (YYYY-MM-DD)
  -file string
        JSON File with user's event data
  -start string
        Start date (YYYY-MM-DD)
  -user string
        GitLab username
```

## Environment Variables

- GITLAB_TOKEN: Personal access token to Gitlab with api permission
- OPENAI_API_KEY: API key for OpenAI / ChatGPT
- GITLAB_DOMAIN: Including scheme, e.g. "https://yourgitlab.com"

## Example

```sh
$ wrought --start 2024-09-01 --end 2024-10-01 --user first.last --count 2
Fetching user events data from GitLab...
Save events to JSON file...
Summarizing events using ChatGPT...

Gitlab Events for user first.last:
- Actively pushed code updates frequently, enhancing project development and progress
- Engaged in in-depth discussions and provided feedback on various drafts and issues like fixing linting problems and adding necessary labels
```