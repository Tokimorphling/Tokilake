import { useCallback, useContext, useEffect, useMemo, useState } from 'react';
import dayjs from 'dayjs';
import 'dayjs/locale/zh-cn';

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

const formatTime = (timestamp) => {
  if (!timestamp) {
    return '永久';
  }
  return dayjs.unix(timestamp).format('YYYY-MM-DD HH:mm:ss');
};

const renderModels = (models = []) => {
  if (!models.length) {
    return <Typography variant="body2" color="text.secondary">暂无模型</Typography>;
  }
  return (
    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
      {models.map((model) => (
        <Chip key={model} label={model} size="small" variant="outlined" />
      ))}
    </Stack>
  );
};

export default function PrivateGroups() {
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

  const loadPageData = useCallback(async () => {
    setLoading(true);
    try {
      const [ownedRes, joinedRes] = await Promise.all([
        API.get('/api/user/private-groups'),
        API.get('/api/user/private-groups/joined')
      ]);
      if (ownedRes.data.success) {
        setOwnedGroups(ownedRes.data.data || []);
      } else {
        showError(ownedRes.data.message);
      }
      if (joinedRes.data.success) {
        setJoinedGroups(joinedRes.data.data || []);
      } else {
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
      if (codesRes.data.success) {
        setInviteCodes(codesRes.data.data || []);
      } else {
        showError(codesRes.data.message);
      }
      if (membersRes.data.success) {
        setMembers(membersRes.data.data || []);
      } else {
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
      showError('请输入分组名');
      return;
    }
    setCreating(true);
    try {
      const res = await API.post('/api/user/private-groups', { group_slug: groupSlug.trim() });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess('私有分组创建成功');
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
      showError('请输入邀请码');
      return;
    }
    setRedeeming(true);
    try {
      const res = await API.post('/api/user/private-groups/redeem', { code: redeemCode.trim() });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      const joinedGroup = res.data.data?.group?.group_slug;
      showSuccess(joinedGroup ? `已加入私有分组 ${joinedGroup}` : '邀请码兑换成功');
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
      showError('请输入新的分组名');
      return;
    }
    try {
      const res = await API.put(`/api/user/private-groups/${selectedGroup.id}`, { group_slug: renameSlug.trim() });
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess('分组重命名成功');
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
    if (!window.confirm(`确认删除私有分组 ${group.group_slug} 吗？删除前请先解绑相关 token 和 TOKIAME_GROUP。`)) {
      return;
    }
    try {
      const res = await API.delete(`/api/user/private-groups/${group.id}`);
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess('分组删除成功');
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
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess('邀请码生成成功');
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
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess(nextStatus === 1 ? '邀请码已启用' : '邀请码已停用');
      await loadManageData(selectedGroup.id);
    } catch (error) {
      showError(error);
    }
  };

  const handleRevokeMember = async (member) => {
    if (!selectedGroup) {
      return;
    }
    if (!window.confirm(`确认撤销 ${member.display_name || member.username || member.user_id} 的分组权限吗？`)) {
      return;
    }
    try {
      const res = await API.delete(`/api/user/private-groups/${selectedGroup.id}/members/${member.user_id}`);
      if (!res.data.success) {
        showError(res.data.message);
        return;
      }
      showSuccess('成员权限已撤销');
      await loadManageData(selectedGroup.id);
    } catch (error) {
      showError(error);
    }
  };

  const ownedColumns = useMemo(
    () => [
      { key: 'group_slug', label: '分组名' },
      { key: 'member_count', label: '成员数' },
      { key: 'invite_code_count', label: '可用邀请码' },
      { key: 'created_at', label: '创建时间' }
    ],
    []
  );

  return (
    <>
      <Stack direction="row" alignItems="center" justifyContent="space-between" mb={5}>
        <Stack direction="column" spacing={1}>
          <Typography variant="h2">私有分组</Typography>
          <Typography variant="subtitle1" color="text.secondary">
            Private Groups
          </Typography>
        </Stack>
      </Stack>

      <Stack spacing={3}>
        <Alert severity="info">
          先创建一个全局唯一的分组名，再把你的 Tokiame 节点 <b>TOKIAME_GROUP</b> 设置成同名值。只有拥有该私有分组或已加入该私有分组的用户才能用它注册 worker；邀请码只负责授权其他用户可见和可用。
        </Alert>

        <Grid container spacing={3}>
          <Grid item xs={12} lg={6}>
            <Card sx={{ p: 3 }}>
              <Stack spacing={2}>
                <Typography variant="h4">创建私有分组</Typography>
                <Typography variant="body2" color="text.secondary">
                  组名只允许小写字母、数字和连字符，长度 3-64。
                </Typography>
                <TextField
                  label="分组名"
                  placeholder="例如 my-tokiame-group"
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
                  创建分组
                </Button>
              </Stack>
            </Card>
          </Grid>
          <Grid item xs={12} lg={6}>
            <Card sx={{ p: 3 }}>
              <Stack spacing={2}>
                <Typography variant="h4">兑换邀请码</Typography>
                <Typography variant="body2" color="text.secondary">
                  兑换成功后，你会立即获得对应私有组的模型可见性和使用权。
                </Typography>
                <TextField
                  label="邀请码"
                  placeholder="ABCD-EFGH-IJKL"
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
                  兑换邀请码
                </Button>
              </Stack>
            </Card>
          </Grid>
        </Grid>

        <Card>
          {loading && <LinearProgress />}
          <Box sx={{ p: 3 }}>
            <Typography variant="h4" mb={2}>
              我创建的私有分组
            </Typography>
            <TableContainer component={Paper} variant="outlined">
              <Table>
                <TableHead>
                  <TableRow>
                    {ownedColumns.map((column) => (
                      <TableCell key={column.key}>{column.label}</TableCell>
                    ))}
                    <TableCell align="right">操作</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {ownedGroups.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={ownedColumns.length + 1} align="center">
                        暂无私有分组
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
                              状态：{group.status === 1 ? '启用' : '停用'}
                            </Typography>
                          </Stack>
                        </TableCell>
                        <TableCell>{group.member_count}</TableCell>
                        <TableCell>{group.invite_code_count}</TableCell>
                        <TableCell>{formatTime(group.created_at)}</TableCell>
                        <TableCell align="right">
                          <Stack direction="row" spacing={1} justifyContent="flex-end">
                            <Tooltip title="管理分组">
                              <IconButton color="primary" onClick={() => openManage(group)}>
                                <Icon icon="solar:widget-4-bold-duotone" />
                              </IconButton>
                            </Tooltip>
                            <Tooltip title="重命名">
                              <IconButton color="secondary" onClick={() => openRename(group)}>
                                <Icon icon="solar:pen-bold-duotone" />
                              </IconButton>
                            </Tooltip>
                            <Tooltip title="删除">
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
              我已加入的私有分组
            </Typography>
            <TableContainer component={Paper} variant="outlined">
              <Table>
                <TableHead>
                  <TableRow>
                    <TableCell>分组名</TableCell>
                    <TableCell>来源</TableCell>
                    <TableCell>加入时间</TableCell>
                    <TableCell>过期时间</TableCell>
                    <TableCell>可用模型</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {joinedGroups.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} align="center">
                        暂无已加入的私有分组
                      </TableCell>
                    </TableRow>
                  ) : (
                    joinedGroups.map((group) => (
                      <TableRow key={group.group_id} hover>
                        <TableCell>{group.group_slug}</TableCell>
                        <TableCell>{group.source || '-'}</TableCell>
                        <TableCell>{formatTime(group.created_at)}</TableCell>
                        <TableCell>{formatTime(group.expires_at)}</TableCell>
                        <TableCell>{renderModels(group.models)}</TableCell>
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
        <DialogTitle>{selectedGroup ? `管理私有分组：${selectedGroup.group_slug}` : '管理私有分组'}</DialogTitle>
        <DialogContent dividers>
          {managing && <LinearProgress sx={{ mb: 2 }} />}
          <Stack spacing={3}>
            <Alert severity="warning">
              邀请码只在创建时展示一次。请在生成后立即复制并发送给目标用户。
            </Alert>

            <Card variant="outlined" sx={{ p: 2 }}>
              <Stack spacing={2}>
                <Typography variant="h5">生成邀请码</Typography>
                <Grid container spacing={2}>
                  <Grid item xs={12} md={4}>
                    <TextField
                      type="number"
                      fullWidth
                      label="最大使用次数"
                      value={inviteMaxUses}
                      onChange={(event) => setInviteMaxUses(Math.max(1, Number(event.target.value) || 1))}
                      inputProps={{ min: 1 }}
                    />
                  </Grid>
                  <Grid item xs={12} md={8}>
                    <LocalizationProvider dateAdapter={AdapterDayjs} adapterLocale="zh-cn">
                      <DateTimePicker
                        label="过期时间（可选）"
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
                  生成邀请码
                </Button>
              </Stack>
            </Card>

            <Box>
              <Typography variant="h5" mb={1.5}>
                邀请码列表
              </Typography>
              <TableContainer component={Paper} variant="outlined">
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>ID</TableCell>
                      <TableCell>状态</TableCell>
                      <TableCell>已用 / 上限</TableCell>
                      <TableCell>过期时间</TableCell>
                      <TableCell>创建时间</TableCell>
                      <TableCell align="right">操作</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {inviteCodes.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6} align="center">
                          暂无邀请码
                        </TableCell>
                      </TableRow>
                    ) : (
                      inviteCodes.map((record) => (
                        <TableRow key={record.id} hover>
                          <TableCell>{record.id}</TableCell>
                          <TableCell>
                            <Chip
                              size="small"
                              label={record.status === 1 ? '启用' : '停用'}
                              color={record.status === 1 ? 'success' : 'default'}
                              variant={record.status === 1 ? 'filled' : 'outlined'}
                            />
                          </TableCell>
                          <TableCell>
                            {record.used_count} / {record.max_uses}
                          </TableCell>
                          <TableCell>{formatTime(record.expires_at)}</TableCell>
                          <TableCell>{formatTime(record.created_at)}</TableCell>
                          <TableCell align="right">
                            <Button
                              size="small"
                              variant="outlined"
                              color={record.status === 1 ? 'warning' : 'success'}
                              onClick={() => handleToggleInviteCode(record)}
                            >
                              {record.status === 1 ? '停用' : '启用'}
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
                成员列表
              </Typography>
              <TableContainer component={Paper} variant="outlined">
                <Table>
                  <TableHead>
                    <TableRow>
                      <TableCell>用户</TableCell>
                      <TableCell>角色</TableCell>
                      <TableCell>来源</TableCell>
                      <TableCell>加入时间</TableCell>
                      <TableCell>过期时间</TableCell>
                      <TableCell align="right">操作</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {members.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={6} align="center">
                          暂无成员
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
                          <TableCell>{member.role}</TableCell>
                          <TableCell>{member.source}</TableCell>
                          <TableCell>{formatTime(member.created_at)}</TableCell>
                          <TableCell>{formatTime(member.expires_at)}</TableCell>
                          <TableCell align="right">
                            {member.role === 'owner' ? (
                              <Chip size="small" label="组主" />
                            ) : (
                              <Button size="small" color="error" variant="outlined" onClick={() => handleRevokeMember(member)}>
                                撤销
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
          <Button onClick={closeManage}>关闭</Button>
        </DialogActions>
      </Dialog>

      <Dialog open={renameVisible} onClose={() => setRenameVisible(false)} maxWidth="sm" fullWidth>
        <DialogTitle>重命名私有分组</DialogTitle>
        <DialogContent dividers>
          <TextField
            fullWidth
            label="新的分组名"
            value={renameSlug}
            onChange={(event) => setRenameSlug(event.target.value)}
            placeholder="例如 my-tokiame-group"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setRenameVisible(false)}>取消</Button>
          <Button onClick={handleRename} variant="contained">
            保存
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={createdInviteCodeVisible} onClose={() => setCreatedInviteCodeVisible(false)} maxWidth="sm" fullWidth>
        <DialogTitle>邀请码已生成</DialogTitle>
        <DialogContent dividers>
          <Stack spacing={2}>
            <Alert severity="success">请立即复制邀请码。关闭后系统不会再次展示明文邀请码。</Alert>
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
                onClick={() => copy(createdInviteCode, '邀请码')}
              >
                复制
              </Button>
            </Paper>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreatedInviteCodeVisible(false)}>关闭</Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
