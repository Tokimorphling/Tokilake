import { useCallback, useContext, useEffect, useMemo, useState } from 'react';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';
import 'dayjs/locale/zh-tw';
import 'dayjs/locale/ja';
import { useTranslation } from 'react-i18next';

import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Grid,
  IconButton,
  LinearProgress,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography
} from '@mui/material';
import { LocalizationProvider } from '@mui/x-date-pickers/LocalizationProvider';
import { AdapterDayjs } from '@mui/x-date-pickers/AdapterDayjs';
import { DateTimePicker } from '@mui/x-date-pickers/DateTimePicker';
import { Icon } from '@iconify/react';

import { API } from 'utils/api';
import { copy, showError, showSuccess } from 'utils/common';
import { UserContext } from 'contexts/UserContext';

const formatTime = (timestamp, t) => {
  if (!timestamp) {
    return t('private_groups.time.permanent');
  }
  return dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm:ss');
};

const renderModels = (models = [], t) => {
  if (!models.length) {
    return (
      <Typography variant="body2" color="text.secondary">
        {t('private_groups.time.no_models')}
      </Typography>
    );
  }
  return (
    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
      {models.map((model) => (
        <Chip key={model} label={model} size="small" variant="outlined" />
      ))}
    </Stack>
  );
};

const isSuccessResponse = (response) => response?.data?.success === true;

export default function PrivateGroups() {
  const { t, i18n } = useTranslation();
  const { loadUserGroup } = useContext(UserContext);
  const [loading, setLoading] = useState(false);
  const [managing, setManaging] = useState(false);
  const [creating, setCreating] = useState(false);
  const [redeeming, setRedeeming] = useState(false);
  const [creatingInviteCode, setCreatingInviteCode] = useState(false);
  const [ownedGroups, setOwnedGroups] = useState([]);
  const [joinedGroups, setJoinedGroups] = useState([]);
  const [inviteCodes, setInviteCodes] = useState([]);
  const [members, setMembers] = useState([]);
  const [groupSlug, setGroupSlug] = useState('');
  const [redeemCode, setRedeemCode] = useState('');
  const [selectedGroup, setSelectedGroup] = useState(null);
  const [manageVisible, setManageVisible] = useState(false);
  const [renameVisible, setRenameVisible] = useState(false);
  const [renameSlug, setRenameSlug] = useState('');
  const [inviteMaxUses, setInviteMaxUses] = useState(1);
  const [inviteExpiresAt, setInviteExpiresAt] = useState(null);
  const [createdInviteCode, setCreatedInviteCode] = useState('');
  const [createdInviteCodeVisible, setCreatedInviteCodeVisible] = useState(false);
  const pickerLocale = useMemo(() => {
    switch (i18n.resolvedLanguage) {
      case 'zh_CN':
        return 'zh-cn';
      case 'zh_HK':
        return 'zh-tw';
      case 'ja_JP':
        return 'ja';
      default:
        return 'en';
    }
  }, [i18n.resolvedLanguage]);

  const loadPageData = useCallback(async () => {
    setLoading(true);
    try {
      const [ownedRes, joinedRes] = await Promise.all([API.get('/api/user/private-groups'), API.get('/api/user/private-groups/joined')]);
      if (isSuccessResponse(ownedRes)) {
        setOwnedGroups(ownedRes.data.data || []);
      } else if (ownedRes?.data?.message) {
        showError(ownedRes.data.message);
      }
      if (isSuccessResponse(joinedRes)) {
        setJoinedGroups(joinedRes.data.data || []);
      } else if (joinedRes?.data?.message) {
        showError(joinedRes.data.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadManageData = useCallback(async (groupId) => {
    setManaging(true);
    try {
      const [codesRes, membersRes] = await Promise.all([
        API.get(`/api/user/private-groups/${groupId}/invite-codes`),
        API.get(`/api/user/private-groups/${groupId}/members`)
      ]);
      if (isSuccessResponse(codesRes)) {
        setInviteCodes(codesRes.data.data || []);
      } else if (codesRes?.data?.message) {
        showError(codesRes.data.message);
      }
      if (isSuccessResponse(membersRes)) {
        setMembers(membersRes.data.data || []);
      } else if (membersRes?.data?.message) {
        showError(membersRes.data.message);
      }
    } catch (error) {
      showError(error);
    } finally {
      setManaging(false);
    }
  }, []);

  useEffect(() => {
    loadPageData();
  }, [loadPageData]);

  const refreshUserGroups = useCallback(() => {
    loadUserGroup();
  }, [loadUserGroup]);

  const handleCreateGroup = async () => {
    if (!groupSlug.trim()) {
      showError(t('private_groups.messages.enter_group_slug'));
      return;
    }
    setCreating(true);
    try {
      const res = await API.post('/api/user/private-groups', { group_slug: groupSlug.trim() });
      if (!isSuccessResponse(res)) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('private_groups.messages.create_success'));
      setGroupSlug('');
      await Promise.all([loadPageData(), refreshUserGroups()]);
    } catch (error) {
      showError(error);
    } finally {
      setCreating(false);
    }
  };

  const handleRedeemCode = async () => {
    if (!redeemCode.trim()) {
      showError(t('private_groups.messages.enter_invite_code'));
      return;
    }
    setRedeeming(true);
    try {
      const res = await API.post('/api/user/private-groups/redeem', { code: redeemCode.trim() });
      if (!isSuccessResponse(res)) {
        showError(res.data.message);
        return;
      }
      const joinedGroup = res.data.data?.group?.group_slug;
      showSuccess(
        joinedGroup
          ? t('private_groups.messages.redeem_success_joined', { slug: joinedGroup })
          : t('private_groups.messages.redeem_success')
      );
      setRedeemCode('');
      await Promise.all([loadPageData(), refreshUserGroups()]);
    } catch (error) {
      showError(error);
    } finally {
      setRedeeming(false);
    }
  };

  const openManage = async (group) => {
    setSelectedGroup(group);
    setManageVisible(true);
    await loadManageData(group.id);
  };

  const closeManage = () => {
    setManageVisible(false);
    setSelectedGroup(null);
    setInviteCodes([]);
    setMembers([]);
    setInviteMaxUses(1);
    setInviteExpiresAt(null);
  };

  const openRename = (group) => {
    setSelectedGroup(group);
    setRenameSlug(group.group_slug);
    setRenameVisible(true);
  };

  const handleRename = async () => {
    if (!selectedGroup) {
      return;
    }
    if (!renameSlug.trim()) {
      showError(t('private_groups.rename.enter_new_slug'));
      return;
    }
    try {
      const res = await API.put(`/api/user/private-groups/${selectedGroup.id}`, { group_slug: renameSlug.trim() });
      if (!isSuccessResponse(res)) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('private_groups.rename.success'));
      setRenameVisible(false);
      if (manageVisible) {
        closeManage();
      }
      await Promise.all([loadPageData(), refreshUserGroups()]);
    } catch (error) {
      showError(error);
    }
  };

  const handleDeleteGroup = async (group) => {
    if (!window.confirm(t('private_groups.messages.delete_confirm', { slug: group.group_slug }))) {
      return;
    }
    try {
      const res = await API.delete(`/api/user/private-groups/${group.id}`);
      if (!isSuccessResponse(res)) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('private_groups.messages.delete_success'));
      if (selectedGroup?.id === group.id) {
        closeManage();
      }
      await Promise.all([loadPageData(), refreshUserGroups()]);
    } catch (error) {
      showError(error);
    }
  };

  const handleCreateInviteCode = async () => {
    if (!selectedGroup) {
      return;
    }
    setCreatingInviteCode(true);
    try {
      const res = await API.post(`/api/user/private-groups/${selectedGroup.id}/invite-codes`, {
        max_uses: inviteMaxUses || 1,
        expires_at: inviteExpiresAt ? Math.floor(inviteExpiresAt.valueOf() / 1000) : 0
      });
      if (!isSuccessResponse(res)) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('private_groups.messages.invite_generate_success'));
      setCreatedInviteCode(res.data.data?.invite_code || '');
      setCreatedInviteCodeVisible(!!res.data.data?.invite_code);
      setInviteMaxUses(1);
      setInviteExpiresAt(null);
      await loadManageData(selectedGroup.id);
    } catch (error) {
      showError(error);
    } finally {
      setCreatingInviteCode(false);
    }
  };

  const handleToggleInviteCode = async (record) => {
    if (!selectedGroup) {
      return;
    }
    const nextStatus = record.status === 1 ? 2 : 1;
    try {
      const res = await API.patch(`/api/user/private-groups/${selectedGroup.id}/invite-codes/${record.id}`, { status: nextStatus });
      if (!isSuccessResponse(res)) {
        showError(res.data.message);
        return;
      }
      showSuccess(nextStatus === 1 ? t('private_groups.manage.status_enabled') : t('private_groups.manage.status_disabled'));
      await loadManageData(selectedGroup.id);
    } catch (error) {
      showError(error);
    }
  };

  const handleRevokeMember = async (member) => {
    if (!selectedGroup) {
      return;
    }
    if (!window.confirm(t('private_groups.manage.revoke_confirm', { user: member.display_name || member.username || member.user_id }))) {
      return;
    }
    try {
      const res = await API.delete(`/api/user/private-groups/${selectedGroup.id}/members/${member.user_id}`);
      if (!isSuccessResponse(res)) {
        showError(res.data.message);
        return;
      }
      showSuccess(t('private_groups.manage.revoke_success'));
      await loadManageData(selectedGroup.id);
    } catch (error) {
      showError(error);
    }
  };

  const ownedColumns = useMemo(
    () => [
      { key: 'group_slug', label: t('private_groups.table.group_slug') },
      { key: 'member_count', label: t('private_groups.table.member_count') },
      { key: 'invite_code_count', label: t('private_groups.table.invite_code_count') },
      { key: 'created_at', label: t('private_groups.table.created_at') }
    ],
    [t]
  );

  return (
    <>
      <Stack direction="row" alignItems="center" justifyContent="space-between" mb={5}>
        <Stack direction="column" spacing={1}>
          <Typography variant="h2">{t('private_groups.title')}</Typography>
          <Typography variant="subtitle1" color="text.secondary">
            {t('private_groups.description')}
          </Typography>
        </Stack>
      </Stack>

      <Stack spacing={3}>
        <Alert severity="info">
          <Box component="span" dangerouslySetInnerHTML={{ __html: t('private_groups.alert_info') }} />
        </Alert>

        <Grid container spacing={3}>
          <Grid item xs={12} lg={6}>
            <Card sx={{ p: 3 }}>
              <Stack spacing={2}>
                <Typography variant="h4">{t('private_groups.create_title')}</Typography>
                <Typography variant="body2" color="text.secondary">
                  {t('private_groups.create_desc')}
                </Typography>
                <TextField
                  label={t('private_groups.group_slug_label')}
                  placeholder={t('private_groups.group_slug_placeholder')}
                  value={groupSlug}
                  onChange={(event) => setGroupSlug(event.target.value)}
                  fullWidth
                />
                <Button
                  variant="contained"
                  startIcon={<Icon icon="solar:add-circle-line-duotone" />}
                  onClick={handleCreateGroup}
                  disabled={creating}
                >
                  {t('private_groups.create_button')}
                </Button>
              </Stack>
            </Card>
          </Grid>
          <Grid item xs={12} lg={6}>
            <Card sx={{ p: 3 }}>
              <Stack spacing={2}>
                <Typography variant="h4">{t('private_groups.redeem_title')}</Typography>
                <Typography variant="body2" color="text.secondary">
                  {t('private_groups.redeem_desc')}
                </Typography>
                <TextField
                  label={t('private_groups.invite_code_label')}
                  placeholder={t('private_groups.invite_code_placeholder')}
                  value={redeemCode}
                  onChange={(event) => setRedeemCode(event.target.value)}
                  fullWidth
                />
                <Button
                  variant="contained"
                  color="secondary"
                  startIcon={<Icon icon="solar:ticket-bold-duotone" />}
                  onClick={handleRedeemCode}
                  disabled={redeeming}
                >
                  {t('private_groups.redeem_button')}
                </Button>
              </Stack>
            </Card>
          </Grid>
        </Grid>

        <Card>
          {loading && <LinearProgress />}
          <Box sx={{ p: 3 }}>
            <Typography variant="h4" mb={2}>
              {t('private_groups.owned_groups_title')}
            </Typography>
            <TableContainer component={Paper} variant="outlined">
              <Table>
                <TableHead>
                  <TableRow>
                    {ownedColumns.map((column) => (
                      <TableCell key={column.key}>{column.label}</TableCell>
                    ))}
                    <TableCell align="right">{t('private_groups.table.actions')}</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {ownedGroups.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={ownedColumns.length + 1} align="center">
                        {t('private_groups.table.no_groups')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    ownedGroups.map((group) => (
                      <TableRow key={group.id} hover>
                        <TableCell>
                          <Stack spacing={0.5}>
                            <Typography variant="body1" fontWeight={600}>
                              {group.group_slug}
                            </Typography>
                            <Typography variant="caption" color="text.secondary">
                              {t('private_groups.table.status')}：
                              {group.status === 1 ? t('private_groups.table.enabled') : t('private_groups.table.disabled')}
                            </Typography>
                          </Stack>
                        </TableCell>
                        <TableCell>{group.member_count}</TableCell>
                        <TableCell>{group.invite_code_count}</TableCell>
                        <TableCell>{formatTime(group.created_at, t)}</TableCell>
                        <TableCell align="right">
                          <Stack direction="row" spacing={1} justifyContent="flex-end">
                            <Tooltip title={t('private_groups.tooltip.manage')}>
                              <IconButton color="primary" onClick={() => openManage(group)}>
                                <Icon icon="solar:widget-4-bold-duotone" />
                              </IconButton>
                            </Tooltip>
                            <Tooltip title={t('private_groups.tooltip.rename')}>
                              <IconButton color="secondary" onClick={() => openRename(group)}>
                                <Icon icon="solar:pen-bold-duotone" />
                              </IconButton>
                            </Tooltip>
                            <Tooltip title={t('private_groups.tooltip.delete')}>
                              <IconButton color="error" onClick={() => handleDeleteGroup(group)}>
                                <Icon icon="solar:trash-bin-trash-bold-duotone" />
                              </IconButton>
                            </Tooltip>
                          </Stack>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>
        </Card>

        <Card>
          {loading && <LinearProgress />}
          <Box sx={{ p: 3 }}>
            <Typography variant="h4" mb={2}>
              {t('private_groups.joined_groups_title')}
            </Typography>
            <TableContainer component={Paper} variant="outlined">
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>{t('private_groups.table.group_slug')}</TableCell>
                    <TableCell>{t('private_groups.table.source')}</TableCell>
                    <TableCell>{t('private_groups.table.joined_at')}</TableCell>
                    <TableCell>{t('private_groups.table.expires_at')}</TableCell>
                    <TableCell>{t('private_groups.table.available_models')}</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {joinedGroups.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} align="center">
                        {t('private_groups.table.no_joined_groups')}
                      </TableCell>
                    </TableRow>
                  ) : (
                    joinedGroups.map((group) => (
                      <TableRow key={group.group_id} hover>
                        <TableCell>{group.group_slug}</TableCell>
                        <TableCell>{group.source || '-'}</TableCell>
                        <TableCell>{formatTime(group.created_at, t)}</TableCell>
                        <TableCell>{formatTime(group.expires_at, t)}</TableCell>
                        <TableCell>{renderModels(group.models, t)}</TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>
        </Card>
      </Stack>

      <Dialog open={manageVisible} onClose={closeManage} maxWidth="lg" fullWidth>
        <DialogTitle>
          {selectedGroup
            ? t('private_groups.manage.title', { slug: selectedGroup.group_slug })
            : t('private_groups.manage.title', { slug: '' })}
        </DialogTitle>
        <DialogContent dividers>
          {managing && <LinearProgress sx={{ mb: 2 }} />}
          <Stack spacing={3}>
            <Alert severity="warning">{t('private_groups.manage.warning')}</Alert>

            <Card variant="outlined" sx={{ p: 2 }}>
              <Stack spacing={2}>
                <Typography variant="h5">{t('private_groups.manage.generate_invite')}</Typography>
                <Grid container spacing={2}>
                  <Grid item xs={12} md={4}>
                    <TextField
                      type="number"
                      fullWidth
                      label={t('private_groups.manage.max_uses')}
                      value={inviteMaxUses}
                      onChange={(event) => setInviteMaxUses(Math.max(1, Number(event.target.value) || 1))}
                      inputProps={{ min: 1 }}
                    />
                  </Grid>
                  <Grid item xs={12} md={8}>
                    <LocalizationProvider dateAdapter={AdapterDayjs} adapterLocale={pickerLocale}>
                      <DateTimePicker
                        label={t('private_groups.manage.expires_at_optional')}
                        value={inviteExpiresAt}
                        onChange={(value) => setInviteExpiresAt(value)}
                        ampm={false}
                        slotProps={{ textField: { fullWidth: true } }}
                      />
                    </LocalizationProvider>
                  </Grid>
                </Grid>
                <Button
                  variant="contained"
                  startIcon={<Icon icon="solar:link-bold-duotone" />}
                  onClick={handleCreateInviteCode}
                  disabled={creatingInviteCode}
                >
                  {t('private_groups.manage.generate_button')}
                </Button>
              </Stack>
            </Card>

            <Box>
              <Typography variant="h5" mb={1.5}>
                {t('private_groups.manage.invite_list')}
              </Typography>
              <TableContainer component={Paper} variant="outlined">
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>{t('private_groups.table.id')}</TableCell>
                      <TableCell>{t('private_groups.table.status')}</TableCell>
                      <TableCell>{t('private_groups.table.used_limit')}</TableCell>
                      <TableCell>{t('private_groups.table.expires_at')}</TableCell>
                      <TableCell>{t('private_groups.table.created_at')}</TableCell>
                      <TableCell align="right">{t('private_groups.table.actions')}</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {inviteCodes.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6} align="center">
                          {t('private_groups.manage.no_invites')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      inviteCodes.map((record) => (
                        <TableRow key={record.id} hover>
                          <TableCell>{record.id}</TableCell>
                          <TableCell>
                            <Chip
                              size="small"
                              label={record.status === 1 ? t('private_groups.table.enabled') : t('private_groups.table.disabled')}
                              color={record.status === 1 ? 'success' : 'default'}
                              variant={record.status === 1 ? 'filled' : 'outlined'}
                            />
                          </TableCell>
                          <TableCell>
                            {record.used_count} / {record.max_uses}
                          </TableCell>
                          <TableCell>{formatTime(record.expires_at, t)}</TableCell>
                          <TableCell>{formatTime(record.created_at, t)}</TableCell>
                          <TableCell align="right">
                            <Button
                              size="small"
                              variant="outlined"
                              color={record.status === 1 ? 'warning' : 'success'}
                              onClick={() => handleToggleInviteCode(record)}
                            >
                              {record.status === 1 ? t('private_groups.table.disabled') : t('private_groups.table.enabled')}
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Box>

            <Box>
              <Typography variant="h5" mb={1.5}>
                {t('private_groups.manage.member_list')}
              </Typography>
              <TableContainer component={Paper} variant="outlined">
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>{t('private_groups.table.user')}</TableCell>
                      <TableCell>{t('private_groups.table.role')}</TableCell>
                      <TableCell>{t('private_groups.table.source')}</TableCell>
                      <TableCell>{t('private_groups.table.joined_at')}</TableCell>
                      <TableCell>{t('private_groups.table.expires_at')}</TableCell>
                      <TableCell align="right">{t('private_groups.table.actions')}</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {members.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6} align="center">
                          {t('private_groups.manage.no_members')}
                        </TableCell>
                      </TableRow>
                    ) : (
                      members.map((member) => (
                        <TableRow key={member.user_id} hover>
                          <TableCell>
                            <Stack spacing={0.5}>
                              <Typography variant="body1">{member.display_name || member.username || member.user_id}</Typography>
                              <Typography variant="caption" color="text.secondary">
                                {member.username || member.user_id}
                              </Typography>
                            </Stack>
                          </TableCell>
                          <TableCell>
                            {member.role === 'owner' ? t('private_groups.table.role_owner') : t('private_groups.table.role_member')}
                          </TableCell>
                          <TableCell>{member.source}</TableCell>
                          <TableCell>{formatTime(member.created_at, t)}</TableCell>
                          <TableCell>{formatTime(member.expires_at, t)}</TableCell>
                          <TableCell align="right">
                            {member.role === 'owner' ? (
                              <Chip size="small" label={t('private_groups.table.role_owner')} />
                            ) : (
                              <Button size="small" color="error" variant="outlined" onClick={() => handleRevokeMember(member)}>
                                {t('private_groups.manage.revoke')}
                              </Button>
                            )}
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            </Box>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={closeManage}>{t('private_groups.manage.close')}</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={renameVisible} onClose={() => setRenameVisible(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{t('private_groups.rename.title')}</DialogTitle>
        <DialogContent dividers>
          <TextField
            fullWidth
            label={t('private_groups.rename.label')}
            value={renameSlug}
            onChange={(event) => setRenameSlug(event.target.value)}
            placeholder={t('private_groups.rename.placeholder')}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setRenameVisible(false)}>{t('private_groups.rename.cancel')}</Button>
          <Button onClick={handleRename} variant="contained">
            {t('private_groups.rename.save')}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={createdInviteCodeVisible} onClose={() => setCreatedInviteCodeVisible(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{t('private_groups.invite_code_dialog.title')}</DialogTitle>
        <DialogContent dividers>
          <Stack spacing={2}>
            <Alert severity="success">{t('private_groups.invite_code_dialog.copy_warning')}</Alert>
            <Paper
              variant="outlined"
              sx={{
                p: 2,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: 2,
                flexWrap: 'wrap'
              }}
            >
              <Typography variant="h5" sx={{ wordBreak: 'break-all' }}>
                {createdInviteCode}
              </Typography>
              <Button
                variant="contained"
                startIcon={<Icon icon="solar:copy-bold-duotone" />}
                onClick={() => copy(createdInviteCode, t('private_groups.invite_code_dialog.copied_name'))}
              >
                {t('private_groups.invite_code_dialog.copy')}
              </Button>
            </Paper>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreatedInviteCodeVisible(false)}>{t('private_groups.invite_code_dialog.close')}</Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
