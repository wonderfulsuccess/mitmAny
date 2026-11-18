import { useEffect, useRef } from "react";
import { ExternalLink, Heart, X } from "lucide-react";
import { tr } from "./i18n";
import { openExternalURL } from "./bridge";

const tiers = [
  { amount: 1, url: "https://buy.stripe.com/aFa7sMaQP5TIgZr7Xt97G02" },
  { amount: 10, url: "https://buy.stripe.com/28E28s3oneqe6kNcdJ97G03" },
  { amount: 100, url: "https://buy.stripe.com/fZu00k3on0zo8sV2D997G04" },
];

export default function SupportDialog({ onClose }: { onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    dialog?.showModal();
    return () => { dialog?.close(); previousFocus?.focus(); };
  }, []);
  return <dialog ref={ref} className="modal support-modal" aria-labelledby="support-title"
    onCancel={event => { event.preventDefault(); onClose(); }}>
    <div className="modal-top">
      <span><Heart size={16} aria-hidden="true" /> {tr("Support mitmAny")}</span>
      <button type="button" aria-label={tr("关闭")} onClick={onClose}><X size={18} /></button>
    </div>
    <h2 id="support-title">{tr("Free software. Powered by your support.")}</h2>
    <p>{tr("Every mitmAny feature is 100% free. If it helps you, an optional tip supports continued maintenance.")}</p>
    <div className="support-tiers">
      {tiers.map(tier => <a key={tier.amount} href={tier.url} target="_blank" rel="noopener noreferrer"
        onClick={event => { event.preventDefault(); openExternalURL(tier.url); }}
        aria-label={tr("Support with US ${p0} — opens Stripe in your browser", {p0: tier.amount})}>
        <strong>US ${tier.amount}</strong><span>{tr("One-time tip")} <ExternalLink size={12} aria-hidden="true" /></span>
      </a>)}
    </div>
    <p className="support-note">{tr("No subscription, paid features or priority support. Stripe checkout opens in your browser; merchant: SimiSF.")}</p>
    <p className="support-note">{tr("Review the currency and total before paying. Alipay and WeChat Pay availability depends on your region and checkout currency.")}</p>
    <p className="support-thanks">{tr("Bug reports, documentation and code contributions help too. Thank you!")}</p>
  </dialog>;
}
