import { Router } from 'express';
import { groupController } from '../controllers/group.controller';
import { sessionAuthGuard } from '../middlewares/auth.middleware';
import { validate } from '../middlewares/validator.middleware';
import { createGroupSchema, groupActionSchema, updateGroupSubjectSchema, updateGroupDescriptionSchema } from '../validators/group.schema';

export const groupRoutes = Router({ mergeParams: true });

groupRoutes.use(sessionAuthGuard);

groupRoutes.post('/', validate(createGroupSchema), groupController.create);
groupRoutes.get('/:jid', groupController.getMetadata);
groupRoutes.post('/:jid/participants/add', validate(groupActionSchema), groupController.addParticipants);
groupRoutes.post('/:jid/participants/remove', validate(groupActionSchema), groupController.removeParticipants);
groupRoutes.post('/:jid/participants/promote', validate(groupActionSchema), groupController.promoteParticipants);
groupRoutes.post('/:jid/participants/demote', validate(groupActionSchema), groupController.demoteParticipants);
groupRoutes.patch('/:jid/subject', validate(updateGroupSubjectSchema), groupController.updateSubject);
groupRoutes.patch('/:jid/description', validate(updateGroupDescriptionSchema), groupController.updateDescription);
groupRoutes.get('/:jid/invite', groupController.getInviteCode);
