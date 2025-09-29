/**
 * 🎨 CLASSIFIED - UI Component Type Definitions
 * Intelligence-grade interface types for cyberpunk components
 */

import { ReactNode, ComponentPropsWithoutRef, ElementType } from 'react';

// ===== COMPONENT BASE TYPES =====

export interface BaseComponentProps {
  readonly className?: string;
  readonly children?: ReactNode;
  readonly id?: string;
  readonly 'data-testid'?: string;
}

export interface VariantProps {
  readonly variant?: ComponentVariant;
  readonly size?: ComponentSize;
  readonly color?: ComponentColor;
}

export interface StateProps {
  readonly loading?: boolean;
  readonly disabled?: boolean;
  readonly error?: boolean;
  readonly success?: boolean;
}

export interface CyberpunkProps {
  readonly glitch?: boolean;
  readonly scanline?: boolean;
  readonly classified?: boolean;
  readonly security_level?: SecurityLevel;
}

// ===== BUTTON TYPES =====

export interface ButtonProps extends 
  BaseComponentProps, 
  VariantProps, 
  StateProps, 
  CyberpunkProps {
  readonly onClick?: () => void;
  readonly type?: 'button' | 'submit' | 'reset';
  readonly fullWidth?: boolean;
  readonly leftIcon?: ReactNode;
  readonly rightIcon?: ReactNode;
}

export interface IconButtonProps extends Omit<ButtonProps, 'leftIcon' | 'rightIcon'> {
  readonly icon: ReactNode;
  readonly 'aria-label': string;
}

// ===== INPUT TYPES =====

export interface InputProps extends 
  BaseComponentProps, 
  VariantProps, 
  StateProps,
  CyberpunkProps {
  readonly value?: string;
  readonly defaultValue?: string;
  readonly placeholder?: string;
  readonly onChange?: (value: string) => void;
  readonly onBlur?: () => void;
  readonly onFocus?: () => void;
  readonly type?: InputType;
  readonly autoComplete?: string;
  readonly required?: boolean;
  readonly readOnly?: boolean;
  readonly maxLength?: number;
  readonly pattern?: string;
  readonly leftAddon?: ReactNode;
  readonly rightAddon?: ReactNode;
}

export interface TextAreaProps extends Omit<InputProps, 'type' | 'leftAddon' | 'rightAddon'> {
  readonly rows?: number;
  readonly cols?: number;
  readonly resize?: 'none' | 'both' | 'horizontal' | 'vertical';
}

export interface SelectProps extends BaseComponentProps, VariantProps, StateProps {
  readonly options: readonly SelectOption[];
  readonly value?: string | readonly string[];
  readonly defaultValue?: string | readonly string[];
  readonly multiple?: boolean;
  readonly searchable?: boolean;
  readonly placeholder?: string;
  readonly onChange?: (value: string | readonly string[]) => void;
}

export interface SelectOption {
  readonly value: string;
  readonly label: string;
  readonly disabled?: boolean;
  readonly description?: string;
  readonly icon?: ReactNode;
}

// ===== LAYOUT TYPES =====

export interface ContainerProps extends BaseComponentProps {
  readonly maxWidth?: ContainerSize;
  readonly padding?: SpacingSize;
  readonly margin?: SpacingSize;
  readonly fluid?: boolean;
}

export interface GridProps extends BaseComponentProps {
  readonly columns?: number | GridColumns;
  readonly gap?: SpacingSize;
  readonly alignItems?: AlignItems;
  readonly justifyContent?: JustifyContent;
  readonly responsive?: boolean;
}

export interface StackProps extends BaseComponentProps {
  readonly direction?: 'row' | 'column';
  readonly spacing?: SpacingSize;
  readonly align?: AlignItems;
  readonly justify?: JustifyContent;
  readonly wrap?: boolean;
}

export interface FlexProps extends BaseComponentProps {
  readonly direction?: FlexDirection;
  readonly wrap?: FlexWrap;
  readonly alignItems?: AlignItems;
  readonly justifyContent?: JustifyContent;
  readonly alignContent?: AlignContent;
  readonly gap?: SpacingSize;
}

// ===== CARD & PANEL TYPES =====

export interface CardProps extends BaseComponentProps, CyberpunkProps {
  readonly title?: string;
  readonly subtitle?: string;
  readonly headerActions?: ReactNode;
  readonly footer?: ReactNode;
  readonly padding?: SpacingSize;
  readonly hoverable?: boolean;
  readonly selectable?: boolean;
  readonly selected?: boolean;
  readonly onSelect?: () => void;
}

export interface PanelProps extends BaseComponentProps, CyberpunkProps {
  readonly title: string;
  readonly collapsible?: boolean;
  readonly collapsed?: boolean;
  readonly onToggle?: (collapsed: boolean) => void;
  readonly actions?: ReactNode;
  readonly status?: PanelStatus;
}

// ===== MODAL & OVERLAY TYPES =====

export interface ModalProps extends BaseComponentProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly title?: string;
  readonly size?: ModalSize;
  readonly closeOnOverlayClick?: boolean;
  readonly closeOnEscape?: boolean;
  readonly preventScroll?: boolean;
  readonly showCloseButton?: boolean;
}

export interface TooltipProps extends BaseComponentProps {
  readonly content: ReactNode;
  readonly placement?: TooltipPlacement;
  readonly trigger?: TooltipTrigger;
  readonly delay?: number;
  readonly arrow?: boolean;
}

export interface PopoverProps extends BaseComponentProps {
  readonly trigger: ReactNode;
  readonly content: ReactNode;
  readonly placement?: PopoverPlacement;
  readonly interactive?: boolean;
  readonly closeOnClickOutside?: boolean;
}

// ===== DATA DISPLAY TYPES =====

export interface TableProps<T = Record<string, unknown>> extends BaseComponentProps {
  readonly data: readonly T[];
  readonly columns: readonly TableColumn<T>[];
  readonly loading?: boolean;
  readonly empty?: ReactNode;
  readonly sortable?: boolean;
  readonly selectable?: boolean;
  readonly selectedRows?: readonly string[];
  readonly onSelectionChange?: (selectedRows: readonly string[]) => void;
  readonly pagination?: PaginationProps;
}

export interface TableColumn<T = Record<string, unknown>> {
  readonly key: string;
  readonly header: string;
  readonly accessor?: keyof T | ((row: T) => unknown);
  readonly cell?: (value: unknown, row: T) => ReactNode;
  readonly sortable?: boolean;
  readonly width?: string | number;
  readonly align?: 'left' | 'center' | 'right';
}

export interface PaginationProps {
  readonly currentPage: number;
  readonly totalPages: number;
  readonly totalItems: number;
  readonly itemsPerPage: number;
  readonly onPageChange: (page: number) => void;
  readonly showQuickJumper?: boolean;
  readonly showSizeChanger?: boolean;
  readonly pageSizeOptions?: readonly number[];
}

// ===== FEEDBACK TYPES =====

export interface AlertProps extends BaseComponentProps, CyberpunkProps {
  readonly type?: AlertType;
  readonly title?: string;
  readonly message: string;
  readonly closable?: boolean;
  readonly onClose?: () => void;
  readonly actions?: ReactNode;
  readonly icon?: ReactNode;
}

export interface ToastProps {
  readonly id: string;
  readonly type: ToastType;
  readonly title?: string;
  readonly message: string;
  readonly duration?: number;
  readonly persistent?: boolean;
  readonly actions?: readonly ToastAction[];
}

export interface ToastAction {
  readonly label: string;
  readonly onClick: () => void;
  readonly variant?: 'primary' | 'secondary';
}

export interface ProgressProps extends BaseComponentProps {
  readonly value: number;
  readonly max?: number;
  readonly label?: string;
  readonly showValue?: boolean;
  readonly indeterminate?: boolean;
  readonly color?: ComponentColor;
  readonly size?: ComponentSize;
  readonly variant?: 'linear' | 'circular';
}

// ===== NAVIGATION TYPES =====

export interface TabsProps extends BaseComponentProps {
  readonly tabs: readonly TabItem[];
  readonly activeTab: string;
  readonly onTabChange: (tabId: string) => void;
  readonly variant?: 'default' | 'pills' | 'underline';
  readonly orientation?: 'horizontal' | 'vertical';
}

export interface TabItem {
  readonly id: string;
  readonly label: string;
  readonly content: ReactNode;
  readonly disabled?: boolean;
  readonly icon?: ReactNode;
  readonly badge?: string | number;
}

export interface BreadcrumbProps extends BaseComponentProps {
  readonly items: readonly BreadcrumbItem[];
  readonly separator?: ReactNode;
  readonly maxItems?: number;
}

export interface BreadcrumbItem {
  readonly label: string;
  readonly href?: string;
  readonly onClick?: () => void;
  readonly icon?: ReactNode;
  readonly current?: boolean;
}

// ===== FORM TYPES =====

export interface FormFieldProps extends BaseComponentProps {
  readonly label?: string;
  readonly description?: string;
  readonly error?: string;
  readonly required?: boolean;
  readonly horizontal?: boolean;
}

export interface FormProps extends BaseComponentProps {
  readonly onSubmit?: (data: Record<string, unknown>) => void;
  readonly onChange?: (data: Record<string, unknown>) => void;
  readonly defaultValues?: Record<string, unknown>;
  readonly validation?: ValidationSchema;
  readonly disabled?: boolean;
}

export interface ValidationSchema {
  readonly [key: string]: ValidationRule[];
}

export interface ValidationRule {
  readonly type: ValidationType;
  readonly message: string;
  readonly value?: unknown;
}

// ===== SPECIALIZED TYPES =====

export interface FileDropzoneProps extends BaseComponentProps {
  readonly accept?: string;
  readonly multiple?: boolean;
  readonly maxSize?: number;
  readonly maxFiles?: number;
  readonly onDrop: (files: readonly File[]) => void;
  readonly onError?: (error: string) => void;
  readonly disabled?: boolean;
  readonly preview?: boolean;
}

export interface CodeBlockProps extends BaseComponentProps {
  readonly code: string;
  readonly language?: string;
  readonly showLineNumbers?: boolean;
  readonly copyable?: boolean;
  readonly theme?: 'dark' | 'light';
  readonly maxHeight?: string | number;
}

export interface SearchInputProps extends Omit<InputProps, 'type'> {
  readonly onSearch?: (query: string) => void;
  readonly suggestions?: readonly string[];
  readonly showHistory?: boolean;
  readonly clearable?: boolean;
}

// ===== ENUM TYPES =====

export type ComponentVariant = 
  | 'primary' 
  | 'secondary' 
  | 'tertiary' 
  | 'ghost' 
  | 'outline' 
  | 'link';

export type ComponentSize = 
  | 'xs' 
  | 'sm' 
  | 'md' 
  | 'lg' 
  | 'xl';

export type ComponentColor = 
  | 'default' 
  | 'primary' 
  | 'secondary' 
  | 'success' 
  | 'warning' 
  | 'error' 
  | 'info';

export type SecurityLevel = 
  | 'unclassified' 
  | 'confidential' 
  | 'secret' 
  | 'top-secret';

export type InputType = 
  | 'text' 
  | 'password' 
  | 'email' 
  | 'number' 
  | 'tel' 
  | 'url' 
  | 'search';

export type ContainerSize = 
  | 'sm' 
  | 'md' 
  | 'lg' 
  | 'xl' 
  | '2xl' 
  | 'full';

export type SpacingSize = 
  | 0 
  | 1 
  | 2 
  | 3 
  | 4 
  | 5 
  | 6 
  | 8 
  | 10 
  | 12 
  | 16 
  | 20 
  | 24;

export type GridColumns = 
  | 1 
  | 2 
  | 3 
  | 4 
  | 5 
  | 6 
  | 7 
  | 8 
  | 9 
  | 10 
  | 11 
  | 12;

export type AlignItems = 
  | 'flex-start' 
  | 'flex-end' 
  | 'center' 
  | 'baseline' 
  | 'stretch';

export type JustifyContent = 
  | 'flex-start' 
  | 'flex-end' 
  | 'center' 
  | 'space-between' 
  | 'space-around' 
  | 'space-evenly';

export type FlexDirection = 
  | 'row' 
  | 'row-reverse' 
  | 'column' 
  | 'column-reverse';

export type FlexWrap = 
  | 'nowrap' 
  | 'wrap' 
  | 'wrap-reverse';

export type AlignContent = 
  | 'flex-start' 
  | 'flex-end' 
  | 'center' 
  | 'space-between' 
  | 'space-around' 
  | 'stretch';

export type PanelStatus = 
  | 'default' 
  | 'success' 
  | 'warning' 
  | 'error' 
  | 'info';

export type ModalSize = 
  | 'sm' 
  | 'md' 
  | 'lg' 
  | 'xl' 
  | 'full';

export type TooltipPlacement = 
  | 'top' 
  | 'top-start' 
  | 'top-end' 
  | 'bottom' 
  | 'bottom-start' 
  | 'bottom-end' 
  | 'left' 
  | 'right';

export type TooltipTrigger = 
  | 'hover' 
  | 'click' 
  | 'focus' 
  | 'manual';

export type PopoverPlacement = TooltipPlacement;

export type AlertType = 
  | 'info' 
  | 'success' 
  | 'warning' 
  | 'error';

export type ToastType = AlertType;

export type ValidationType = 
  | 'required' 
  | 'minLength' 
  | 'maxLength' 
  | 'pattern' 
  | 'email' 
  | 'url' 
  | 'number' 
  | 'custom';

// ===== UTILITY TYPES =====

export type PolymorphicComponentProps<E extends ElementType, P = {}> = P & 
  Omit<ComponentPropsWithoutRef<E>, keyof P> & {
    as?: E;
  };

export type WithRequired<T, K extends keyof T> = T & Required<Pick<T, K>>;

export type WithOptional<T, K extends keyof T> = Omit<T, K> & Partial<Pick<T, K>>;

export type DeepPartial<T> = {
  [P in keyof T]?: T[P] extends object ? DeepPartial<T[P]> : T[P];
};

export type DeepReadonly<T> = {
  readonly [P in keyof T]: T[P] extends object ? DeepReadonly<T[P]> : T[P];
};
