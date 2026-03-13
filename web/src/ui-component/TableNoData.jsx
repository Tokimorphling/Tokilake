import PropTypes from 'prop-types';
import { Box, Typography, TableRow, TableCell } from '@mui/material';

import { useTranslation } from 'react-i18next';

const TableNoData = ({ message }) => {
  const { t } = useTranslation();
  const displayMessage = message || t('dashboard_index.no_data_available');
  return (
    <TableRow>
      <TableCell colSpan={1000}>
        <Box
          sx={{
            minHeight: '490px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center'
          }}
        >
          <Typography variant="h3" color={'#697586'}>
            {displayMessage}
          </Typography>
        </Box>
      </TableCell>
    </TableRow>
  );
};
export default TableNoData;

TableNoData.propTypes = {
  message: PropTypes.string
};
