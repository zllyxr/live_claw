-- Backend v2: move legacy admin-owned support conversations into the seat queue.
--
-- Before the independent console existed, an administrator could become the
-- assignee without having a support-agent profile. Such conversations must not
-- disappear from every ordinary seat's queue after the split.

UPDATE support_conversations conversation
LEFT JOIN support_agents agent
  ON agent.admin_user_id=conversation.assigned_admin_id AND agent.status=1
SET conversation.status=0,
    conversation.assigned_admin_id=0,
    conversation.assigned_at=NULL
WHERE conversation.status IN (0,1)
  AND conversation.assigned_admin_id<>0
  AND agent.admin_user_id IS NULL;

UPDATE support_conversations conversation
JOIN support_agents agent
  ON agent.admin_user_id=conversation.assigned_admin_id AND agent.status=1
SET conversation.status=1,
    conversation.assigned_at=COALESCE(conversation.assigned_at,conversation.updated_at)
WHERE conversation.status IN (0,1)
  AND conversation.assigned_admin_id<>0;
