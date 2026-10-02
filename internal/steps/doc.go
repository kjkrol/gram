// Package engine runs the steps of rules and plans: the nodes a rule's Moment and a plan's Actor
// build, laid out as a tree, run at a moment within a plugin's pass (Instant) or over the steps
// of the world's simulation (Plans). Package rule and package rule/plan are its faces; nothing
// else imports it.
package steps
