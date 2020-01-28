import styled from 'styled-components';
import React from 'react';
import { Icon } from 'evergreen-ui';

import theme from '../../theme';
import Text from './text';
import { Row, Box } from './box';

const StyledSelect = styled(Box).attrs({ as: 'select' })`
  background: ${props => props.theme.colors.black};
  color: ${props => props.theme.colors.white};
  border-radius: ${props => props.theme.radii[1]}px;
  appearance: none;
  padding: 8px;
  font-size: 16px;
  font-weight: 300;
  display: flex;
  flex: 1;
  border: 1px solid ${props => props.theme.colors.white};
  outline: none;

  transition: ${props => props.theme.transition};

  &:focus {
    border-color: ${props => props.theme.colors.primary};
  }
`;

const Select = ({
  searchable,
  multi,
  creatable,
  options,
  disabled,
  autoFocus,
  value,
  required,
  placeholder,
  none = 'There are no options',
  onChange,
  ...props
}) => {
  return (
    <Row flex={1} position="relative" style={{ cursor: 'pointer' }} {...props}>
      <StyledSelect
        multiple={multi}
        disabled={disabled}
        autoFocus={autoFocus}
        required={required}
        value={value}
        onChange={onChange}
      >
        {options.length === 0 && (
          <option value="" disabled selected hidden>
            {none}
          </option>
        )}
        {options.length > 0 && placeholder && (
          <option value="" disabled selected hidden>
            {placeholder}
          </option>
        )}
        {options.map(({ label, value }) => (
          <option value={value}>{label}</option>
        ))}
      </StyledSelect>
      <Icon
        icon="caret-down"
        color={theme.colors.white}
        size={18}
        style={{ position: 'absolute', right: 8, top: 8 }}
        pointerEvents="none"
      />
    </Row>
  );
};

export default Select;
