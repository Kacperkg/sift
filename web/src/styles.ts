import styles from './styles.module.css'

export { styles }

export function cx(...classNames: Array<string | false | null | undefined>) {
	return classNames.filter(Boolean).map(className => styles[className as string]).join(' ')
}
