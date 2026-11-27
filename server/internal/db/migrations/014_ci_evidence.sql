-- Only CI reports CI: agents may not attach ci.* evidence, which the gate takes as the CI result for a head commit.
-- Before this, the default "anyone attaches evidence" rule let the worker vouch for its own CI. The GitHub webhook
-- principal keeps it; humans are not agents and keep it too.
INSERT INTO policies(name, subject, action, effect, priority)
SELECT n, s, a, e, p FROM (VALUES
  ('agents do not report CI', 'agent', 'task.evidence:ci.*', 'deny', 100),
  ('the GitHub webhook reports CI', 'principal:github', 'task.evidence:ci.*', 'allow', 200)
) AS r(n, s, a, e, p) WHERE EXISTS (SELECT 1 FROM policies);
