import { LockIcon } from './Icons'
import { styles } from '../styles'

export function PrivacyFooter() {
  return <footer className={styles.footer}>
    <div>
      <LockIcon />
      <span><strong>Your inbox stays yours.</strong> Email is read locally and job records are encrypted with a key held in macOS Keychain.</span>
    </div>
    <span>JOB INBOX / PRIVATE INDEX</span>
  </footer>
}
