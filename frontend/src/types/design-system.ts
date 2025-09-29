/**
 * 🎨 CLASSIFIED - INQTEL Design System Types
 * TypeScript interfaces for cyberpunk intelligence components
 */

// ===== DESIGN TOKENS =====

export type ClassificationLevel = 
  | 'unclassified' 
  | 'confidential' 
  | 'secret' 
  | 'top-secret'

export type CyberColor = 
  | 'primary'      // Matrix green
  | 'secondary'    // Electric blue  
  | 'accent'       // Neon pink
  | 'warning'      // Alert amber
  | 'danger'       // Threat red
  | 'success'      // Terminal green

export type CyberSize = 
  | 'xs' 
  | 'sm' 
  | 'md' 
  | 'lg' 
  | 'xl'

export type CyberVariant = 
  | 'primary' 
  | 'secondary' 
  | 'ghost' 
  | 'outline'

// ===== COMPONENT INTERFACES =====

export interface CyberBaseProps {
  readonly className?: string
  readonly id?: string
  readonly 'data-testid'?: string
  readonly children?: React.ReactNode
}

export interface CyberClassificationProps {
  readonly classification?: ClassificationLevel
  readonly showBadge?: boolean
  readonly restricted?: boolean
}

export interface CyberEffectProps {
  readonly glow?: boolean
  readonly glitch?: boolean
  readonly scanline?: boolean
  readonly pulse?: boolean
  readonly matrixRain?: boolean
}

// ===== BUTTON SYSTEM =====

export interface CyberButtonProps extends CyberBaseProps, CyberEffectProps {
  readonly variant?: CyberVariant
  readonly size?: CyberSize
  readonly color?: CyberColor
  readonly fullWidth?: boolean
  readonly loading?: boolean
  readonly disabled?: boolean
  readonly leftIcon?: React.ReactNode
  readonly rightIcon?: React.ReactNode
  readonly onClick?: () => void
  readonly type?: 'button' | 'submit' | 'reset'
}

export interface CyberIconButtonProps extends Omit<CyberButtonProps, 'leftIcon' | 'rightIcon'> {
  readonly icon: React.ReactNode
  readonly 'aria-label': string
}

// ===== TERMINAL SYSTEM =====

export interface CyberTerminalProps extends CyberBaseProps, CyberClassificationProps {
  readonly title?: string
  readonly subtitle?: string
  readonly headerActions?: React.ReactNode
  readonly collapsible?: boolean
  readonly collapsed?: boolean
  readonly onToggle?: (collapsed: boolean) => void
}

export interface CyberTerminalHeaderProps extends CyberBaseProps {
  readonly title: string
  readonly subtitle?: string
  readonly classification?: ClassificationLevel
  readonly actions?: React.ReactNode
  readonly onClose?: () => void
}

export interface CyberTerminalBodyProps extends CyberBaseProps {
  readonly padding?: CyberSize
  readonly scrollable?: boolean
}

// ===== DATA DISPLAY =====

export interface CyberDataGridProps<T = Record<string, unknown>> extends CyberBaseProps {
  readonly data: readonly T[]
  readonly columns: readonly CyberDataColumn<T>[]
  readonly loading?: boolean
  readonly empty?: React.ReactNode
  readonly sortable?: boolean
  readonly selectable?: boolean
  readonly selectedRows?: readonly string[]
  readonly onSelectionChange?: (selectedRows: readonly string[]) => void
  readonly classification?: ClassificationLevel
}

export interface CyberDataColumn<T = Record<string, unknown>> {
  readonly key: string
  readonly header: string
  readonly accessor?: keyof T | ((row: T) => unknown)
  readonly cell?: (value: unknown, row: T) => React.ReactNode
  readonly sortable?: boolean
  readonly width?: string | number
  readonly align?: 'left' | 'center' | 'right'
  readonly classification?: ClassificationLevel
}

export interface CyberDataRowProps extends CyberBaseProps {
  readonly selected?: boolean
  readonly onClick?: () => void
  readonly classification?: ClassificationLevel
}

// ===== PROGRESS INDICATORS =====

export interface CyberProgressProps extends CyberBaseProps, CyberEffectProps {
  readonly value: number
  readonly max?: number
  readonly label?: string
  readonly showValue?: boolean
  readonly indeterminate?: boolean
  readonly color?: CyberColor
  readonly size?: CyberSize
  readonly variant?: 'linear' | 'circular'
  readonly classification?: ClassificationLevel
}

export interface CyberStatusIndicatorProps extends CyberBaseProps {
  readonly status: 'online' | 'offline' | 'warning' | 'error' | 'classified'
  readonly label?: string
  readonly pulse?: boolean
  readonly size?: CyberSize
}

// ===== INPUT SYSTEM =====

export interface CyberInputProps extends CyberBaseProps, CyberEffectProps {
  readonly value?: string
  readonly defaultValue?: string
  readonly placeholder?: string
  readonly onChange?: (value: string) => void
  readonly onBlur?: () => void
  readonly onFocus?: () => void
  readonly type?: 'text' | 'password' | 'email' | 'search' | 'number'
  readonly size?: CyberSize
  readonly disabled?: boolean
  readonly readOnly?: boolean
  readonly error?: boolean
  readonly success?: boolean
  readonly leftAddon?: React.ReactNode
  readonly rightAddon?: React.ReactNode
  readonly classification?: ClassificationLevel
}

export interface CyberTextAreaProps extends Omit<CyberInputProps, 'type' | 'leftAddon' | 'rightAddon'> {
  readonly rows?: number
  readonly cols?: number
  readonly resize?: 'none' | 'both' | 'horizontal' | 'vertical'
}

export interface CyberSelectProps extends CyberBaseProps {
  readonly options: readonly CyberSelectOption[]
  readonly value?: string | readonly string[]
  readonly defaultValue?: string | readonly string[]
  readonly multiple?: boolean
  readonly searchable?: boolean
  readonly placeholder?: string
  readonly onChange?: (value: string | readonly string[]) => void
  readonly classification?: ClassificationLevel
}

export interface CyberSelectOption {
  readonly value: string
  readonly label: string
  readonly disabled?: boolean
  readonly description?: string
  readonly icon?: React.ReactNode
  readonly classification?: ClassificationLevel
}

// ===== LAYOUT SYSTEM =====

export interface CyberPanelProps extends CyberBaseProps, CyberClassificationProps, CyberEffectProps {
  readonly variant?: 'default' | 'terminal' | 'classified' | 'alert'
  readonly size?: CyberSize
  readonly padding?: CyberSize
  readonly border?: boolean
  readonly shadow?: boolean
}

export interface CyberGridProps extends CyberBaseProps {
  readonly columns?: number | 'auto'
  readonly gap?: CyberSize
  readonly alignItems?: 'start' | 'center' | 'end' | 'stretch'
  readonly justifyContent?: 'start' | 'center' | 'end' | 'between' | 'around' | 'evenly'
  readonly responsive?: boolean
}

export interface CyberStackProps extends CyberBaseProps {
  readonly direction?: 'row' | 'column'
  readonly spacing?: CyberSize
  readonly align?: 'start' | 'center' | 'end' | 'stretch'
  readonly justify?: 'start' | 'center' | 'end' | 'between' | 'around' | 'evenly'
  readonly wrap?: boolean
}

// ===== FEEDBACK SYSTEM =====

export interface CyberAlertProps extends CyberBaseProps, CyberClassificationProps {
  readonly type?: 'info' | 'success' | 'warning' | 'error' | 'classified'
  readonly title?: string
  readonly message: string
  readonly closable?: boolean
  readonly onClose?: () => void
  readonly actions?: React.ReactNode
  readonly icon?: React.ReactNode
  readonly persistent?: boolean
}

export interface CyberToastProps {
  readonly id: string
  readonly type: 'info' | 'success' | 'warning' | 'error' | 'classified'
  readonly title?: string
  readonly message: string
  readonly duration?: number
  readonly persistent?: boolean
  readonly classification?: ClassificationLevel
  readonly actions?: readonly CyberToastAction[]
}

export interface CyberToastAction {
  readonly label: string
  readonly onClick: () => void
  readonly variant?: 'primary' | 'secondary'
}

export interface CyberModalProps extends CyberBaseProps, CyberClassificationProps {
  readonly isOpen: boolean
  readonly onClose: () => void
  readonly title?: string
  readonly size?: 'sm' | 'md' | 'lg' | 'xl' | 'full'
  readonly closeOnOverlayClick?: boolean
  readonly closeOnEscape?: boolean
  readonly preventScroll?: boolean
  readonly showCloseButton?: boolean
  readonly terminal?: boolean
}

// ===== NAVIGATION SYSTEM =====

export interface CyberTabsProps extends CyberBaseProps {
  readonly tabs: readonly CyberTabItem[]
  readonly activeTab: string
  readonly onTabChange: (tabId: string) => void
  readonly variant?: 'default' | 'terminal' | 'classified'
  readonly orientation?: 'horizontal' | 'vertical'
  readonly classification?: ClassificationLevel
}

export interface CyberTabItem {
  readonly id: string
  readonly label: string
  readonly content: React.ReactNode
  readonly disabled?: boolean
  readonly icon?: React.ReactNode
  readonly badge?: string | number
  readonly classification?: ClassificationLevel
}

export interface CyberBreadcrumbProps extends CyberBaseProps {
  readonly items: readonly CyberBreadcrumbItem[]
  readonly separator?: React.ReactNode
  readonly maxItems?: number
  readonly classification?: ClassificationLevel
}

export interface CyberBreadcrumbItem {
  readonly label: string
  readonly href?: string
  readonly onClick?: () => void
  readonly icon?: React.ReactNode
  readonly current?: boolean
  readonly classification?: ClassificationLevel
}

// ===== SPECIALIZED COMPONENTS =====

export interface CyberFileDropzoneProps extends CyberBaseProps, CyberClassificationProps {
  readonly accept?: string
  readonly multiple?: boolean
  readonly maxSize?: number
  readonly maxFiles?: number
  readonly onDrop: (files: readonly File[]) => void
  readonly onError?: (error: string) => void
  readonly disabled?: boolean
  readonly preview?: boolean
  readonly terminal?: boolean
}

export interface CyberCodeBlockProps extends CyberBaseProps {
  readonly code: string
  readonly language?: string
  readonly showLineNumbers?: boolean
  readonly copyable?: boolean
  readonly theme?: 'dark' | 'matrix' | 'terminal'
  readonly maxHeight?: string | number
  readonly classification?: ClassificationLevel
}

export interface CyberSearchInputProps extends Omit<CyberInputProps, 'type'> {
  readonly onSearch?: (query: string) => void
  readonly suggestions?: readonly string[]
  readonly showHistory?: boolean
  readonly clearable?: boolean
  readonly terminal?: boolean
}

export interface CyberMetricsCardProps extends CyberBaseProps, CyberClassificationProps {
  readonly title: string
  readonly value: string | number
  readonly unit?: string
  readonly trend?: 'up' | 'down' | 'neutral'
  readonly trendValue?: string | number
  readonly icon?: React.ReactNode
  readonly color?: CyberColor
  readonly sparkline?: readonly number[]
}

export interface CyberStatusBoardProps extends CyberBaseProps {
  readonly metrics: readonly CyberMetric[]
  readonly alerts?: readonly CyberAlert[]
  readonly classification?: ClassificationLevel
  readonly realTime?: boolean
  readonly refreshInterval?: number
}

export interface CyberMetric {
  readonly id: string
  readonly label: string
  readonly value: string | number
  readonly unit?: string
  readonly status: 'normal' | 'warning' | 'critical'
  readonly trend?: 'up' | 'down' | 'neutral'
  readonly history?: readonly number[]
}

export interface CyberAlert {
  readonly id: string
  readonly level: 'info' | 'warning' | 'error' | 'critical'
  readonly title: string
  readonly message: string
  readonly timestamp: string
  readonly source: string
  readonly classification?: ClassificationLevel
  readonly acknowledged?: boolean
}

// ===== THEME SYSTEM =====

export interface CyberTheme {
  readonly colors: CyberColorPalette
  readonly typography: CyberTypography
  readonly spacing: CyberSpacing
  readonly effects: CyberEffects
  readonly classification: CyberClassificationTheme
}

export interface CyberColorPalette {
  readonly primary: string
  readonly secondary: string
  readonly accent: string
  readonly warning: string
  readonly danger: string
  readonly success: string
  readonly void: string
  readonly panel: string
  readonly surface: string
  readonly text: {
    readonly primary: string
    readonly secondary: string
    readonly tertiary: string
    readonly disabled: string
  }
}

export interface CyberTypography {
  readonly display: string
  readonly mono: string
  readonly ui: string
  readonly sizes: Record<CyberSize, string>
  readonly weights: Record<string, number>
}

export interface CyberSpacing {
  readonly scale: Record<string, string>
  readonly components: Record<string, string>
}

export interface CyberEffects {
  readonly glows: Record<CyberColor, string>
  readonly animations: Record<string, string>
  readonly transitions: Record<string, string>
}

export interface CyberClassificationTheme {
  readonly colors: Record<ClassificationLevel, string>
  readonly backgrounds: Record<ClassificationLevel, string>
  readonly effects: Record<ClassificationLevel, string>
}

// ===== UTILITY TYPES =====

export type CyberComponentProps<T extends keyof JSX.IntrinsicElements> = 
  CyberBaseProps & 
  Omit<JSX.IntrinsicElements[T], keyof CyberBaseProps>

export type WithClassification<T> = T & CyberClassificationProps

export type WithEffects<T> = T & CyberEffectProps

export type CyberStyleVariant<T extends string> = {
  readonly [K in T]: string
}

// ===== CONTEXT TYPES =====

export interface CyberThemeContextValue {
  readonly theme: CyberTheme
  readonly classification: ClassificationLevel
  readonly setClassification: (level: ClassificationLevel) => void
  readonly effectsEnabled: boolean
  readonly setEffectsEnabled: (enabled: boolean) => void
}
