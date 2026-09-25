// Iconos por nombre (los mismos nombres que usa la API y packages/design/icons.json).
// Importación explícita: el paquete solo incluye lo que se usa.
import {
  Armchair, Banknote, Beef, Beer, CakeSlice, Camera, Check, ChefHat, Cherry, ChevronLeft, ChevronRight, CircleAlert,
  CircleCheck, Citrus, Clock, CloudOff, Coffee, Cookie, CookingPot, Croissant, Crown, CupSoda, Dessert, Donut,
  Drumstick, EggFried, Eye, EyeOff, Fish, Flame, GlassWater, GripVertical, Ham, House, IceCreamCone, ImagePlus,
  Images, Info, KeyRound, LayoutGrid, LogOut, type LucideIcon, Mail, MapPin, Martini, Milk, Minus, Monitor, Pencil,
  Percent, Pizza, Plus, Popcorn, Printer, QrCode, Receipt, Rocket, Salad, Sandwich, Search, Settings, ShieldCheck,
  Snowflake, Soup, Sparkles, Sprout, Store, Tag, Trash2, TrendingUp, Upload, Users, Utensils, UtensilsCrossed,
  Wifi, WifiOff, Wine, X, Zap,
} from "lucide-react";

export const ICONOS: Record<string, LucideIcon> = {
  // comida (categorías)
  utensils: Utensils, "utensils-crossed": UtensilsCrossed, "cooking-pot": CookingPot, "chef-hat": ChefHat, flame: Flame,
  beef: Beef, drumstick: Drumstick, fish: Fish, ham: Ham, salad: Salad, soup: Soup, sandwich: Sandwich, pizza: Pizza,
  "egg-fried": EggFried, croissant: Croissant, sprout: Sprout, popcorn: Popcorn, "cake-slice": CakeSlice,
  "ice-cream-cone": IceCreamCone, dessert: Dessert, donut: Donut, cookie: Cookie, cherry: Cherry, coffee: Coffee,
  milk: Milk, "cup-soda": CupSoda, citrus: Citrus, "glass-water": GlassWater, beer: Beer, wine: Wine, martini: Martini,
  // estaciones (nombres semánticos del sistema de diseño)
  cocina: Flame, bar: Wine, cocinaFria: Snowflake, kds: Monitor, factura: Receipt, impresora: Printer, caja: Banknote,
  // interfaz
  inicio: House, salon: LayoutGrid, personal: Users, ajustes: Settings, salir: LogOut, agregar: Plus, buscar: Search,
  cerrar: X, ok: Check, chevronRight: ChevronRight, chevronLeft: ChevronLeft, camara: Camera, galeria: Images,
  subir: Upload, fotoNueva: ImagePlus, eliminar: Trash2, editar: Pencil, correo: Mail, ver: Eye, ocultar: EyeOff,
  sinInternet: WifiOff, enLinea: Wifi, magia: Sparkles, tienda: Store, mesa: Armchair, arrastrar: GripVertical,
  etiqueta: Tag, exito: CircleCheck, error: CircleAlert, info: Info, pin: KeyRound, seguro: ShieldCheck,
  tiempo: Clock, ubicacion: MapPin, porcentaje: Percent, nube: CloudOff, cohete: Rocket, qr: QrCode, menos: Minus,
  dueno: Crown, tendencia: TrendingUp, rayo: Zap, camera: Camera, armchair: Armchair, users: Users, printer: Printer,
  receipt: Receipt,
};

export function Icon({ name, size = 20, strokeWidth = 2, className }: { name: string; size?: number; strokeWidth?: number; className?: string }) {
  const C = ICONOS[name] ?? Utensils;
  return <C size={size} strokeWidth={strokeWidth} className={className} aria-hidden="true" />;
}
